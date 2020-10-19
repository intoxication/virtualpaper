package process

import (
	"errors"
	"fmt"
	"github.com/otiai10/gosseract"
	"github.com/sirupsen/logrus"
	"gopkg.in/gographics/imagick.v3/imagick"
	"os"
	"path"
	"strings"
	"time"
	"tryffel.net/go/virtualpaper/config"
	"tryffel.net/go/virtualpaper/models"
	"tryffel.net/go/virtualpaper/storage"
)

type fileProcessor struct {
	*Task
	document *models.Document
	input    chan fileOp
	file     string
	rawFile  *os.File
	tempFile *os.File
}

func newFileProcessor(id int, db *storage.Database) *fileProcessor {
	fp := &fileProcessor{
		Task:  newTask(id, db),
		input: make(chan fileOp, 5),
	}
	fp.idle = true
	fp.runFunc = fp.waitEvent
	return fp
}

func (fp *fileProcessor) waitEvent() {
	timer := time.NewTimer(time.Millisecond * 50)
	select {
	case <-timer.C:
		// pass

	case fileOp := <-fp.input:
		fp.process(fileOp)
		//fp.processFile()

		//fp.processFile()
	}
}

func (fp *fileProcessor) process(op fileOp) {
	if op.document == nil && op.file != "" {
		fp.processFile()
	} else if op.document != nil {
		fp.document = op.document
		fp.processDocument()
	} else {
		logrus.Warningf("process task got empty fileop, skipping")
	}
}

func (fp *fileProcessor) processDocument() {

	pendingSteps, err := fp.db.JobStore.GetDocumentPendingSteps(fp.document.Id)
	if err != nil {
		logrus.Errorf("get pending processing steps for document %d: %v", fp.document.Id, err)
		return
	}

	file, err := os.Open(path.Join(config.C.Processing.DocumentsDir, fp.document.Hash))
	if err != nil {
		logrus.Errorf("open document %d file: %v", fp.document.Id, err)
		return
	}

	for _, step := range *pendingSteps {
		switch step.Step {
		case models.ProcessHash:
			err := fp.updateHash(fp.document, file)
			if err != nil {
				logrus.Errorf("update hash: %v", err)
				return
			}
		case models.ProcessThumbnail:
			err := fp.generateThumbnail(file)
			if err != nil {
				logrus.Errorf("generate thumbnail: %v", err)
				return
			}
		case models.ProcessParseContent:
			err := fp.parseContent()
			if err != nil {
				logrus.Errorf("generate thumbnail: %v", err)
				return
			}
		default:
			logrus.Warningf("unhandle process step: %v, skipping", step.Step)
		}
	}

	file.Close()
}

// re-calculate hash. If it differs from current document.Hash, update document record and rename file to new hash,
// if different.
func (fp *fileProcessor) updateHash(doc *models.Document, file *os.File) error {
	process := &models.ProcessItem{
		DocumentId: fp.document.Id,
		Step:       models.ProcessHash,
		CreatedAt:  time.Now(),
	}

	job, err := fp.db.JobStore.StartProcessItem(process, "calculate hash")
	if err != nil {
		return fmt.Errorf("persist process item: %v", err)
	}

	defer fp.persistProcess(process, job)
	hash, err := getHash(file)
	if err != nil {
		job.Status = models.JobFailure
		return err
	}

	if hash != doc.Hash {
		logrus.Infof("rename file %s to %s", doc.Hash, hash)
	} else {
		logrus.Infof("file hash has not changed")
		job.Status = models.JobFinished
		job.Message = "hash: no change"
		return nil
	}

	oldName := file.Name()
	err = os.Rename(oldName, path.Join(config.C.Processing.DocumentsDir, hash))
	if err != nil {
		job.Status = models.JobFailure
		return fmt.Errorf("rename file (doc %d) by old hash: %v", fp.document.Id, err)
	}

	fp.document.Hash = hash
	err = fp.db.DocumentStore.Update(fp.document)
	if err != nil {
		job.Status = models.JobFailure
		return fmt.Errorf("save updated document: %v", err)
	}

	job.Status = models.JobFinished
	return nil
}

func (fp *fileProcessor) updateThumbnail(doc *models.Document, file *os.File) error {
	imagick.Initialize()
	defer imagick.Terminate()

	process := &models.ProcessItem{
		DocumentId: fp.document.Id,
		Step:       models.ProcessThumbnail,
		CreatedAt:  time.Now(),
	}

	job, err := fp.db.JobStore.StartProcessItem(process, "generate thumbnail")
	if err != nil {
		return fmt.Errorf("persist process item: %v", err)
	}
	job.Message = "Generate thumbnail"
	defer fp.persistProcess(process, job)

	output := path.Join(config.C.Processing.PreviewsDir, fp.document.Hash+".png")

	logrus.Infof("generate thumbnail for document %d", fp.document.Id)
	_, err = imagick.ConvertImageCommand([]string{
		"convert", "-thumbnail", "x500", "-background", "white", "-alpha", "remove", file.Name() + "[0]", output,
	})

	err = fp.db.DocumentStore.Update(doc)
	if err != nil {
		logrus.Errorf("update document record: %v", err)
	}

	if err != nil {
		job.Status = models.JobFailure
		job.Message += "; " + err.Error()
		return fmt.Errorf("call imagick: %v", err)
	}
	job.Status = models.JobFinished
	return nil
}

func (fp *fileProcessor) processFile() {
	logrus.Infof("task %d, process file %s", fp.id, fp.file)

	fp.lock.Lock()
	fp.idle = false
	fp.lock.Unlock()
	var err error

	fp.rawFile, err = os.OpenFile(fp.file, os.O_RDONLY, os.ModePerm)

	defer fp.cleanup()

	if err != nil {
		logrus.Errorf("process file %s, open: %v", fp.file, err)
		return
	}

	duplicate, err := fp.isDuplicate()
	if duplicate {
		logrus.Infof("file %s is a duplicate, ignore file", fp.file)
		return
	}

	if err != nil {
		logrus.Errorf("get duplicate status: %v", err)
		return
	}

	err = fp.createNewDocumentRecord()
	if err != nil {
		logrus.Error(err)
		return
	}

	logrus.Info("generate thumbnail")
	err = fp.generateThumbnail(fp.rawFile)
	if err != nil {
		logrus.Error("generate thumbnail: %v", err)
		return
	}

	logrus.Info("parse content")
	err = fp.parseContent()
	if err != nil {
		logrus.Errorf("Parse document content: %v", err)
	}
}

func (fp *fileProcessor) cleanup() {
	logrus.Infof("Stop processing file %s", fp.file)
	fp.document = nil
	if fp.rawFile != nil {
		fp.rawFile.Close()
		fp.rawFile = nil
	}
	if fp.tempFile != nil {
		fp.tempFile.Close()

		err := os.Remove(fp.tempFile.Name())
		if err != nil {
			logrus.Errorf("remove temp file %s: %v", fp.tempFile.Name(), err)
		}
		fp.tempFile = nil
	}
	fp.file = ""
	fp.lock.Lock()
	fp.idle = true
	fp.lock.Unlock()
}

func (fp *fileProcessor) isDuplicate() (bool, error) {
	hash, err := getHash(fp.rawFile)
	if err != nil {
		return false, err
	}

	document, err := fp.db.DocumentStore.GetByHash(hash)
	if err != nil {
		if errors.Is(err, storage.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	if document != nil {
		return true, nil
	}
	return false, nil
}

func (fp *fileProcessor) createNewDocumentRecord() error {
	_, fileName := path.Split(fp.file)

	doc := &models.Document{
		Id:       0,
		UserId:   5,
		Name:     fileName,
		Content:  "",
		Filename: fileName,
	}

	var err error
	doc.Hash, err = getHash(fp.rawFile)
	if err != nil {
		return fmt.Errorf("get hash: %v", err)
	}

	err = fp.db.DocumentStore.Create(doc)
	if err != nil {
		return fmt.Errorf("store document: %v", err)
	}

	fp.document = doc
	return nil
}

func (fp *fileProcessor) generateThumbnail(file *os.File) error {
	imagick.Initialize()
	defer imagick.Terminate()

	process := &models.ProcessItem{
		DocumentId: fp.document.Id,
		Step:       models.ProcessThumbnail,
		CreatedAt:  time.Now(),
	}

	job, err := fp.db.JobStore.StartProcessItem(process, "calculate hash")
	if err != nil {
		return fmt.Errorf("persist process item: %v", err)
	}
	defer fp.persistProcess(process, job)

	output := path.Join(config.C.Processing.PreviewsDir, fp.document.Hash+".png")

	_, err = imagick.ConvertImageCommand([]string{
		"convert", "-thumbnail", "x500", "-background", "white", "-alpha", "remove", file.Name() + "[0]", output,
	})

	if err != nil {
		job.Status = models.JobFailure
		job.Message += "; " + err.Error()
		return fmt.Errorf("call imagick: %v", err)
	}

	job.Status = models.JobFinished
	return nil
}

func (fp *fileProcessor) parseContent() error {
	// if pdf, generate image preview and pass it to tesseract
	var imageFile string
	var err error

	if strings.HasSuffix(strings.ToLower(fp.file), "pdf") {
		job := &models.Job{
			DocumentId: fp.document.Id,
			Message:    "Render image from pdf content",
			Status:     models.JobAwaiting,
			StartedAt:  time.Now(),
			StoppedAt:  time.Now(),
		}

		err := fp.db.JobStore.Create(fp.document.Id, job)
		if err != nil {
			logrus.Warningf("create job record: %v", err)
		}

		imagick.Initialize()
		defer imagick.Terminate()

		imageFile = path.Join(config.C.Processing.TmpDir, fp.document.Hash+".png")
		_, err = imagick.ConvertImageCommand([]string{
			"convert", "-density", "300", fp.file, "-depth", "8", imageFile,
		})
		if err != nil {
			job.Message += "; " + err.Error()
			job.Status = models.JobFailure
			fp.persistJob(job)
			return err
		} else {
			job.Status = models.JobFinished

		}
		fp.persistJob(job)
	}

	client := gosseract.NewClient()
	defer client.Close()

	job := &models.Job{
		DocumentId: fp.document.Id,
		Message:    "Parse content with tesseract",
		Status:     models.JobAwaiting,
		StartedAt:  time.Now(),
		StoppedAt:  time.Now(),
	}

	err = fp.db.JobStore.Create(fp.document.Id, job)
	if err != nil {
		logrus.Warningf("create job record: %v", err)
	}
	defer fp.persistJob(job)

	err = client.SetImage(imageFile)
	if err != nil {
		return fmt.Errorf("set ocr image source: %v", err)
	}

	text, err := client.Text()
	if err != nil {
		job.Message += "; " + err.Error()
		job.Status = models.JobFailure
		return fmt.Errorf("parse document text: %v", err)
	} else {
		fp.document.Content = text

		err = fp.db.DocumentStore.SetDocumentContent(fp.document.Id, text)
		if err != nil {
			job.Message += "; " + "save document content: " + err.Error()
			job.Status = models.JobFailure
			return fmt.Errorf("save document content: %v", err)
		} else {
			job.Status = models.JobFinished
		}
	}
	return nil
}

func (fp *fileProcessor) persistProcess(process *models.ProcessItem, job *models.Job) {
	err := fp.db.JobStore.MarkProcessingDone(process, job.Status == models.JobFinished)
	if err != nil {
		logrus.Errorf("mark process complete: %v", err)
	}
	job.StoppedAt = time.Now()
	err = fp.db.JobStore.Update(job)
	if err != nil {
		logrus.Errorf("save job to database: %v", err)
	}

}

func (fp *fileProcessor) persistJob(job *models.Job) {
	job.StoppedAt = time.Now()
	err := fp.db.JobStore.Update(job)
	if err != nil {
		logrus.Errorf("save job to database: %v", err)
	}
}
