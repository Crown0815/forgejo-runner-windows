// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"os"
)

const registrationWarning = "This file is automatically generated by act-runner. Do not edit it manually unless you know what you are doing. Removing this file will cause act runner to re-register as a new runner."

// Registration is the registration information for a runner
type Registration struct {
	Warning string `json:"WARNING"` // Warning message to display, it's always the registrationWarning constant

	ID      int64    `json:"id"`
	UUID    string   `json:"uuid"`
	Name    string   `json:"name"`
	Token   string   `json:"token"`
	Address string   `json:"address"`
	Labels  []string `json:"labels"`
}

func LoadRegistration(file string) (*Registration, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reg Registration
	if err := json.NewDecoder(f).Decode(&reg); err != nil {
		return nil, err
	}

	reg.Warning = ""

	return &reg, nil
}

func SaveRegistration(file string, reg *Registration) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	reg.Warning = registrationWarning

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(reg)
}