//  Copyright (c) 2012 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package db

import (
	"encoding/json"
	"log"
)

type ChannelRemoval struct {
	Seq uint64 `json:"seq"`
	Rev string `json:"rev"`
}
type ChannelMap map[string]*ChannelRemoval

// Maps usernames to lists of channel names.
type AccessMap map[string][]string

// The sync-gateway metadata stored in the "_sync" property of a Couchbase document.
type syncData struct {
	ID         string     `json:"id"`
	CurrentRev string     `json:"rev"`
	Sequence   uint64     `json:"sequence"`
	History    RevTree    `json:"history"`
	Channels   ChannelMap `json:"channels,omitempty"`
	Access     AccessMap  `json:"access,omitempty"`
}

// A document as stored in Couchbase. Contains the body of the current revision plus metadata.
// In its JSON form, the body's properties are at top-level while the syncData is in a special
// "_sync" property.
type document struct {
	syncData
	body Body
}

// Returns a new empty document.
func newDocument() *document {
	return &document{syncData: syncData{History: make(RevTree)}}
}

// Fetches the body of a revision as a map, or nil if it's not available.
func (doc *document) getRevision(revid string) Body {
	var body Body
	if revid == doc.CurrentRev {
		body = doc.body
	} else {
		body = doc.History.getParsedRevisionBody(revid)
		if body == nil {
			return nil
		}
	}
	body["_id"] = doc.ID
	body["_rev"] = revid
	return body
}

// Fetches the body of a revision as JSON, or nil if it's not available.
func (doc *document) getRevisionJSON(revid string) []byte {
	var bodyJSON []byte
	if revid == doc.CurrentRev {
		bodyJSON, _ = json.Marshal(doc.body)
	} else {
		bodyJSON, _ = doc.History.getRevisionBody(revid)
	}
	return bodyJSON
}

// Adds a revision body to a document.
func (doc *document) setRevision(revid string, body Body) {
	strippedBody := stripSpecialProperties(body)
	if revid == doc.CurrentRev {
		doc.body = strippedBody
	} else {
		var asJson []byte
		if len(body) > 0 {
			asJson, _ = json.Marshal(stripSpecialProperties(body))
		}
		doc.History.setRevisionBody(revid, asJson)
	}
}

//////// MARSHALING ////////

type documentRoot struct {
	SyncData *syncData `json:"_sync"`
}

func (doc *document) UnmarshalJSON(data []byte) error {
	root := documentRoot{SyncData: &syncData{History: make(RevTree)}}
	err := json.Unmarshal([]byte(data), &root)
	if err != nil {
		log.Printf("error unmarshaling documentRoot: %s", err)
		return err
	}
	if root.SyncData != nil {
		doc.syncData = *root.SyncData
	}

	err = json.Unmarshal([]byte(data), &doc.body)
	if err != nil {
		log.Printf("error unmarshaling body: %s", err)
		return err
	}
	delete(doc.body, "_sync")
	return nil
}

func (doc *document) MarshalJSON() ([]byte, error) {
	body := doc.body
	if body == nil {
		body = Body{}
	}
	body["_sync"] = &doc.syncData
	data, err := json.Marshal(body)
	delete(body, "_sync")
	return data, err
}
