/*
 * Virtualpaper is a service to manage users paper documents in virtual format.
 * Copyright (C) 2022  Tero Vierimaa
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import React from "react";
import {
  Edit,
  TextInput,
  DateField,
  TextField,
  useAuthProvider,
  email,
  Labeled,
  SaveButton,
  Form,
  Button,
} from "react-admin";

import {
  Typography,
  Grid,
  InputLabel,
  OutlinedInput,
  InputAdornment,
  IconButton,
  Tooltip,
  Paper,
  AccordionSummary,
  Accordion,
  AccordionDetails,
} from "@mui/material";
import Visibility from "@mui/icons-material/Visibility";
import VisibilityOff from "@mui/icons-material/VisibilityOff";
import { Link } from "react-router-dom";
import { StopWordsInput, SynonymsInput } from "./Settings";
import { ExpandMore } from "@mui/icons-material";

export const ProfileEdit = (staticContext: any, ...props: any) => {
  return (
    <Edit
      redirect={false}
      id="user"
      resource="preferences"
      basePath="/preferences"
      title="Profile"
      {...props}
    >
      <Form warnWhenUnsavedChanges>
        <Paper sx={{ p: 3 }}>
          <Grid container width={{ xs: "100%", xl: 800 }} spacing={2}>
            <Grid item xs={12} md={8}>
              <Typography variant="h5">User settings</Typography>
            </Grid>
            <Grid item xs={12} md={8}>
              <Labeled label="User id">
                <TextField source="user_id" />
              </Labeled>
            </Grid>
            <Grid item xs={12} md={8}>
              <TextInput source="email" validate={email()} />
            </Grid>
            <Grid item xs={12} md={8}>
              <Labeled label="Username">
                <TextField source="user_name" />
              </Labeled>
            </Grid>
            <Grid item xs={12} md={8}>
              <Labeled label="Joined at">
                <DateField source="created_at" />
              </Labeled>
              <span style={{ marginLeft: 20 }} />

              <Labeled label="Settings last changed at">
                <DateField source="updated_at" />
              </Labeled>
            </Grid>
            <Grid item xs={12} md={8}>
              <ShowToken />
            </Grid>
            <Grid item xs={12} md={8} sx={{ mt: 4 }}>
              <Typography variant="h5">Statistics</Typography>
              <Labeled label="Number of documents">
                <TextField source="documents_count" label={"Documents count"} />
              </Labeled>
              <span style={{ marginLeft: 20 }} />
              <Labeled label="Total size of all documents">
                <TextField
                  source="documents_size_string"
                  label={"Total size of documents"}
                />
              </Labeled>
            </Grid>
            <Grid item></Grid>
            <Grid item xs={12} md={4}>
              <Typography variant="h4">Search customization</Typography>
              <Accordion>
                <AccordionSummary expandIcon={<ExpandMore />}>
                  <Typography variant="h6">Stop words</Typography>
                </AccordionSummary>
                <AccordionDetails style={{ flexDirection: "column" }}>
                  <Typography variant="body2">
                    Stop words are words that are excluded when ranking
                    documents during search query. They will not modify the
                    documents in any way, they are only meant to improve the
                    relevancy of search results.
                  </Typography>
                  <Typography variant="body2">
                    Format: one stop word per line
                  </Typography>
                  <StopWordsInput />
                </AccordionDetails>
              </Accordion>
              <Accordion>
                <AccordionSummary expandIcon={<ExpandMore />}>
                  <Typography variant="h6">Synonyms</Typography>
                </AccordionSummary>
                <AccordionDetails style={{ flexDirection: "column" }}>
                  <Typography variant="body2">
                    Synonyms are words that are treated as same when searching
                    documents. They will not modify the contents of documents in
                    any way, they will only improve the relevancy of the search
                    results.
                  </Typography>
                  <Typography variant="body2">
                    Format: list of synonyms separated by comma, e.g.
                    ('food','spaghetti','pasta')
                  </Typography>
                  <SynonymsInput />
                </AccordionDetails>
              </Accordion>
            </Grid>
            <Grid item xs={12} sx={{ m: 3 }} justifyContent={"flex-end"}>
              <Grid container justifyContent={"space-between"}>
                <ProfileEditActions />
                <ResetPasswordButton />
              </Grid>
            </Grid>
          </Grid>
        </Paper>
      </Form>
    </Edit>
  );
};

const ProfileEditActions = () => {
  return <SaveButton />;
};

const ResetPasswordButton = () => {
  return (
    <Link
      to={"/auth/forgot-password"}
      style={{ fontSize: 16, textDecoration: "none", paddingLeft: 2 }}
    >
      <Button size="small">
        <span>Set password</span>
      </Button>
    </Link>
  );
};

const ShowToken = () => {
  const authProvider = useAuthProvider();
  const token = authProvider.getToken();
  const [tokenShown, setTokenShown] = React.useState(false);

  const handleClickShowPassword = () => {
    setTokenShown(!tokenShown);
  };

  const handleMouseDownPassword = (
    event: React.MouseEvent<HTMLButtonElement>
  ) => {
    event.preventDefault();
  };

  return (
    <>
      <InputLabel htmlFor="outlined-adornment-password">API Token</InputLabel>
      <Tooltip title="Api token. Please read documentation first. This will grant access to all user data, so please be careful not to expose it.">
        <OutlinedInput
          multiline
          id="outlined-adornment-password"
          type={tokenShown ? "text" : "password"}
          value={tokenShown ? token : "******"}
          endAdornment={
            <InputAdornment position="end">
              <IconButton
                aria-label="toggle password visibility"
                onClick={handleClickShowPassword}
                onMouseDown={handleMouseDownPassword}
                edge="end"
              >
                {tokenShown ? <VisibilityOff /> : <Visibility />}
              </IconButton>
            </InputAdornment>
          }
          label="Password"
        />
      </Tooltip>
    </>
  );
};

export default {
  edit: ProfileEdit,
};
