// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package run

import (
	"io"

	log "github.com/sirupsen/logrus"
)

// NullLogger is used to create a new JobLogger to discard logs. This
// will prevent these logs from being logged to the stdout, but
// forward them to the Reporter via its hook.
type NullLogger struct{}

// WithJobLogger creates a new logrus.Logger that will discard all logs.
func (n NullLogger) WithJobLogger() *log.Logger {
	logger := log.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(log.TraceLevel)

	return logger
}
