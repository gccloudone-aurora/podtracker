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
	"github.com/gccloudone-aurora/podtracker/internal/tracking"
)

// BackendWriter is an interface that describes what functions of a BackendWriter implementation should have
type BackendWriter interface {
	// Write takes PodInfo and writes/sends/publishes it to some backend store that implements the interface.
	//
	// Common implementations might include:
	//   - HTTPS / API -based write
	//   - Simply writing to stdout
	Write(*tracking.PodInfo) error
}

// WriteToAll writes the provided PodInfo to all configured backends
func WriteToAll(writers []BackendWriter, info *tracking.PodInfo) []error {
	errors := []error{}

	// write using all the configured backend writers
	for _, writer := range writers {
		if err := writer.Write(info); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}
