# Contributing

([Français](#comment-contribuer))

## How to Contribute

When contributing, post comments and discuss changes you wish to make via Issues.

Feel free to propose changes by creating Pull Requests. If you don't have write access, editing a file will create a Fork of this project for you to save your proposed changes to. Submitting a change to a file will write it to a new Branch in your Fork, so you can send a Pull Request.

If this is your first time contributing on GitHub, don't worry! Let us know if you have any questions.

## Running Locally

See the [Running Locally](/README.md#running-locally) documentation

## Adding a Custom Backend Writer

Backend Writers are special types that implement the [`BackendWriter`](/internal/writer/writer.go) interface. By implementing this interface, you can extend the functionality of PodTracker with additional methods for logging of [`PodInfo`](/internal/tracking/tracking.go).

To implement a new Backend Writer, first we need to add the writer to the [`writer`](/internal/writer/) package and create the struct for the Backend Writer type and implement the [`BackendWriter`](/internal/writer/writer.go) interface.

For example, take a look at the [`stdout`](/internal/writer/stdout.go) writer

```go

```

Our new writer is now implemented as a usable type, but we still can't actually use it. Lucky for us, since this is a Kubernetes operator that follows the [`Operator Pattern`](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/), we can configure writers via CRD. In order to do so, however, we need a way to pass down the configuration for a new writer implementation to the operator.

create `your_writer.go` inside of the [`internal/writer`](/internal/writer/) folder

```go
package writer

import (
	"encoding/json"
	"os"

	"github.com/gccloudone-aurora/podtracker/internal/tracking"
)

// A backend writer for writing PodInfo to YourBackend
type YourWriter struct {
	enabled bool
	// any additional fields that describe information needed for your writer to function correctly ...
}

// A blank assignment to ensure that YourWriter implements BackendWriter
var _ BackendWriter = &YourWriter{}

// Implement the BackendWriter interface
func (w YourWriter) Write(info *tracking.PodInfo) error {
	// TODO: implement writer logic
	// this logic should describe a step-by-step process for formatting and writing data contained in the provided `info` argument to your backend service/store
}

// YourWriterConfig is a concrete way to configure the StdoutWriter
type YourWriterConfig struct {
	Enabled bool `json:"enabled"`
	// configuration you would like passed in through the PodTracker CRD when operators configure your writer
}

// NewYourWriter creates and configures a YourWriter and returns a reference to it
func NewYourWriter(cfg *YourWriterConfig) *YourWriter {
	// TODO: do some stuff ...

	// return the concrete implementation of your writer
	return &YourWriter{enabled: cfg.Enabled}
}
```

Then update the backend writer config to include your new writer configuration

```go
package writer

type BackendWriterConfig struct {
	Stdout *StdoutConfig `json:"stdout,omitempty"`
  YourWriter *YourWriterConfig `json:"yourWriter,omitempty"`
}

// GetWriters will create a list of concrete BackendWriter based which writers are configured in the incoming BackendWriterConfig
func (b *BackendWriterConfig) GetWriters() ([]BackendWriter, error) {
	writers := []BackendWriter{}

	// ...

	if b.YourWriter != nil {
		writers = append(writers, NewYourWriter(b.YourWriter))
	}

	// ...

	return writers
}

```

Finally, ensure the deepcopy implementations and kubernetes manifests are up-to-date

```bash
make generate && make manifests
```

Now, your custom writer can be configured and used with PodTracker by adding its configuration to the PodTracker spec

```yaml
apiVersion: networking.aurora.gc.ca/v1
kind: PodTracker
metadata:
  name: podtracker-your-writer
spec:
	nsToWatch:
	- '*-system'
	backendWriterConfig:
		yourWriter:
			enabled: true
			# ...
```

### Security

**Do not post any security issues on the public repository!** See [SECURITY.md](SECURITY.md)

______________________

## Comment contribuer

Lorsque vous contribuez, veuillez également publier des commentaires et discuter des modifications que vous souhaitez apporter par l'entremise des enjeux (Issues).

N'hésitez pas à proposer des modifications en créant des demandes de tirage (Pull Requests). Si vous n'avez pas accès au mode de rédaction, la modification d'un fichier créera une copie (Fork) de ce projet afin que vous puissiez enregistrer les modifications que vous proposez. Le fait de proposer une modification à un fichier l'écrira dans une nouvelle branche dans votre copie (Fork), de sorte que vous puissiez envoyer une demande de tirage (Pull Request).

Si c'est la première fois que vous contribuez à GitHub, ne vous en faites pas! Faites-nous part de vos questions.

### Sécurité

**Ne publiez aucun problème de sécurité sur le dépôt publique!** Voir [SECURITY.md](SECURITY.md)
