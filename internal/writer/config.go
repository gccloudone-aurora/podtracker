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

//
// CONFIGURATION
//

// BackendWriterConfig defines configuration options for backends that can be used by PodTracker to
// write Pod tracking info to all configured backends
type BackendWriterConfig struct {
	Stdout *StdoutConfig `json:"stdout,omitempty"`
	// NOTE: more to be added as desired
}

// GetWriters will create a list of concrete BackendWriter based which writers are configured in the incoming BackendWriterConfig
func (b BackendWriterConfig) GetWriters() []BackendWriter {
	writers := []BackendWriter{}
	if b.Stdout != nil {
		writers = append(writers, NewStdoutWriter(b.Stdout))
	}

	// check if any writers were configured
	// if not, just use stdout writer as a default
	if len(writers) == 0 {
		return []BackendWriter{NewStdoutWriter(nil)}
	}

	return writers
}
