// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/snapcore/snapd/asserts"
	"github.com/snapcore/snapd/client"
	"github.com/snapcore/snapd/overlord/auth"
	"github.com/snapcore/snapd/overlord/devicestate"
	"github.com/snapcore/snapd/overlord/state"
)

var (
	serialModelCmd = &Command{
		Path:       "/v2/model/serial",
		GET:        getSerial,
		ReadAccess: openAccess{},
	}
	modelCmd = &Command{
		Path:        "/v2/model",
		POST:        postModel,
		GET:         getModel,
		ReadAccess:  openAccess{},
		WriteAccess: rootAccess{},
	}
)

var devicestateRemodel = devicestate.Remodel

type postModelData struct {
	NewModel string `json:"new-model"`
}

type modelAssertJSON struct {
	Headers map[string]interface{} `json:"headers,omitempty"`
	Body    string                 `json:"body,omitempty"`
}

func postModel(c *Command, r *http.Request, _ *auth.UserState) Response {
	defer r.Body.Close()
	var data postModelData
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return BadRequest("cannot decode request body into remodel operation: %v", err)
	}
	rawNewModel, err := asserts.Decode([]byte(data.NewModel))
	if err != nil {
		return BadRequest("cannot decode new model assertion: %v", err)
	}
	newModel, ok := rawNewModel.(*asserts.Model)
	if !ok {
		return BadRequest("new model is not a model assertion: %v", newModel.Type())
	}

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	chg, err := devicestateRemodel(st, newModel)
	if err != nil {
		return BadRequest("cannot remodel device: %v", err)
	}
	ensureStateSoon(st)

	return AsyncResponse(nil, &Meta{Change: chg.ID()})

}

// getModel gets the current model assertion using the DeviceManager
func getModel(c *Command, r *http.Request, _ *auth.UserState) Response {
	opts, err := parseHeadersFormatOptionsFromURL(r.URL.Query())
	if err != nil {
		return BadRequest(err.Error())
	}

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	devmgr := c.d.overlord.DeviceManager()

	model, err := devmgr.Model()
	if err == state.ErrNoState {
		res := &errorResult{
			Message: "no model assertion yet",
			Kind:    client.ErrorKindAssertionNotFound,
			Value:   "model",
		}

		return &resp{
			Type:   ResponseTypeError,
			Result: res,
			Status: 404,
		}
	}
	if err != nil {
		return InternalError("accessing model failed: %v", err)
	}

	if opts.jsonResult {
		modelJSON := modelAssertJSON{}

		modelJSON.Headers = model.Headers()
		if !opts.headersOnly {
			modelJSON.Body = string(model.Body())
		}

		return SyncResponse(modelJSON, nil)
	}

	return AssertResponse([]asserts.Assertion{model}, false)
}

// getSerial gets the current serial assertion using the DeviceManager
func getSerial(c *Command, r *http.Request, _ *auth.UserState) Response {
	opts, err := parseHeadersFormatOptionsFromURL(r.URL.Query())
	if err != nil {
		return BadRequest(err.Error())
	}

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	devmgr := c.d.overlord.DeviceManager()

	serial, err := devmgr.Serial()
	if err == state.ErrNoState {
		res := &errorResult{
			Message: "no serial assertion yet",
			Kind:    client.ErrorKindAssertionNotFound,
			Value:   "serial",
		}

		return &resp{
			Type:   ResponseTypeError,
			Result: res,
			Status: 404,
		}
	}
	if err != nil {
		return InternalError("accessing serial failed: %v", err)
	}

	if opts.jsonResult {
		serialJSON := modelAssertJSON{}

		serialJSON.Headers = serial.Headers()
		if !opts.headersOnly {
			serialJSON.Body = string(serial.Body())
		}

		return SyncResponse(serialJSON, nil)
	}

	return AssertResponse([]asserts.Assertion{serial}, false)
}
