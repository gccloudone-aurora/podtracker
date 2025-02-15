/*

MIT License

Copyright (c) His Majesty the King in Right of Canada, as represented by the
Minister responsible for Shared Services Canada, 2024

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

package writer

import (
	"encoding/json"
	"os"

	"github.com/gccloudone-aurora/podtracker/internal/tracking"
)

// A backend writer for writing PodInfo to stdout
type StdoutWriter struct {
	enabled bool
}

// A blank assignment to ensure that StdoutWriter implements BackendWriter
var _ BackendWriter = &StdoutWriter{}

// Implement the BackendWriter interface
func (s StdoutWriter) Write(info *tracking.PodInfo) error {
	if !s.enabled {
		return nil
	}

	resp, err := json.Marshal(info)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(append(resp, byte('\n')))
	return err
}

// StdoutConfig is a concrete way to configure the StdoutWriter
type StdoutConfig struct {
	Enabled bool `json:"enabled"`
}

// NewStdoutWriter creates and configures a StdoutWriter and returns a reference to it
func NewStdoutWriter(cfg *StdoutConfig) *StdoutWriter {
	if cfg == nil {
		return &StdoutWriter{enabled: true}
	}
	return &StdoutWriter{enabled: cfg.Enabled}
}
