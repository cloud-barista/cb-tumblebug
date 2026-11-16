/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package model

import "encoding/json"

// The post-deployment script was requested as postCommands / postCommandAsync before it became
// postScript with the postScriptAsync query parameter. Requests, templates and stored records written
// with the old names are still read here; output always uses the new names. A request that gives both
// is read by its postScript.

// legacyPostScript holds the old request field names
type legacyPostScript struct {
	Phases []PostScriptPhase `json:"postCommands"`
	Async  bool              `json:"postCommandAsync"`
}

// merge returns cur, or the old fields as a PostScriptReq when cur is not set
func (l legacyPostScript) merge(cur *PostScriptReq) *PostScriptReq {
	if cur != nil || len(l.Phases) == 0 {
		return cur
	}
	return &PostScriptReq{Phases: l.Phases}
}

// UnmarshalJSON also reads the old postCommands fields
func (r *InfraReq) UnmarshalJSON(b []byte) error {
	type plain InfraReq
	aux := struct {
		*plain
		legacyPostScript
	}{plain: (*plain)(r)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	r.PostScript = aux.merge(r.PostScript)
	r.PostScriptAsync = r.PostScriptAsync || aux.Async
	return nil
}

// UnmarshalJSON also reads the old postCommands fields
func (r *InfraDynamicReq) UnmarshalJSON(b []byte) error {
	type plain InfraDynamicReq
	aux := struct {
		*plain
		legacyPostScript
	}{plain: (*plain)(r)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	r.PostScript = aux.merge(r.PostScript)
	r.PostScriptAsync = r.PostScriptAsync || aux.Async
	return nil
}

// UnmarshalJSON also reads the old postCommands fields
func (r *AddNodeGroupDynamicReq) UnmarshalJSON(b []byte) error {
	type plain AddNodeGroupDynamicReq
	aux := struct {
		*plain
		legacyPostScript
	}{plain: (*plain)(r)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	r.PostScript = aux.merge(r.PostScript)
	r.PostScriptAsync = r.PostScriptAsync || aux.Async
	return nil
}

// UnmarshalJSON also reads the old postCommands field
func (a *AutoAction) UnmarshalJSON(b []byte) error {
	type plain AutoAction
	aux := struct {
		*plain
		legacyPostScript
	}{plain: (*plain)(a)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	a.PostScript = aux.merge(a.PostScript)
	return nil
}

// UnmarshalJSON also reads the old postCommands field
func (r *InfraAutopilotReq) UnmarshalJSON(b []byte) error {
	type plain InfraAutopilotReq
	aux := struct {
		*plain
		legacyPostScript
	}{plain: (*plain)(r)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	r.PostScript = aux.merge(r.PostScript)
	r.PostScriptAsync = r.PostScriptAsync || aux.Async
	return nil
}

// UnmarshalJSON also reads the old policyOnPostCommandFailure field
func (r *BuildAgnosticImageReq) UnmarshalJSON(b []byte) error {
	type plain BuildAgnosticImageReq
	aux := struct {
		*plain
		LegacyPolicy string `json:"policyOnPostCommandFailure"`
	}{plain: (*plain)(r)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if r.PolicyOnPostScriptFailure == "" {
		r.PolicyOnPostScriptFailure = aux.LegacyPolicy
	}
	return nil
}

// UnmarshalJSON also reads the old postCommand* fields of records stored before the rename
func (i *InfraInfo) UnmarshalJSON(b []byte) error {
	type plain InfraInfo
	aux := struct {
		*plain
		legacyPostScript
		LegacyResults   []PostScriptPhaseResult `json:"postCommandResults"`
		LegacyStatus    PostScriptStatus        `json:"postCommandStatus"`
		LegacyRequestId string                  `json:"postCommandRequestId"`
	}{plain: (*plain)(i)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	i.PostScript = aux.merge(i.PostScript)
	if i.PostScriptResults == nil {
		i.PostScriptResults = aux.LegacyResults
	}
	if i.PostScriptStatus == "" {
		i.PostScriptStatus = aux.LegacyStatus
	}
	if i.PostScriptRequestId == "" {
		i.PostScriptRequestId = aux.LegacyRequestId
	}
	return nil
}

// UnmarshalJSON reads the embedded InfraInfo and the autopilot fields separately: without it the
// decoder promoted from InfraInfo would take the whole object and leave the autopilot fields empty
func (r *InfraAutopilotResult) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &r.InfraInfo); err != nil {
		return err
	}
	rest := struct {
		NodeSpecResults      *[]NodeSpecResult      `json:"nodeSpecResults"`
		ProvisioningAttempts *[]ProvisioningAttempt `json:"provisioningAttempts"`
		AutopilotStats       *AutopilotStats        `json:"autopilotStats"`
	}{&r.NodeSpecResults, &r.ProvisioningAttempts, &r.AutopilotStats}
	return json.Unmarshal(b, &rest)
}
