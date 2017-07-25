package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deis/acid/pkg/acid"
	"github.com/deis/quokka/pkg/javascript/libk8s"
)

const DefaultVCSSidecar = "acidic.azurecr.io/vcs-sidecar:latest"

// store represents a storage engine for a acid.Project.
type store struct{}

// New initializes a new storage backend.
func New() *store { return new(store) }

// Get retrieves the project from storage.
func (s store) Get(id, namespace string) (*acid.Project, error) {
	return loadProjectConfig(projectID(id), namespace)
}

// projectID will encode a project name.
func projectID(id string) string {
	if strings.HasPrefix(id, "acid-") {
		return id
	}
	return "acid-" + ShortSHA(id)
}

// loadProjectConfig loads a project config from inside of Kubernetes.
//
// The namespace is the namespace where the secret is stored.
func loadProjectConfig(id, namespace string) (*acid.Project, error) {
	kc, err := libk8s.KubeClient()
	proj := &acid.Project{}
	if err != nil {
		return proj, err
	}

	// The project config is stored in a secret.
	secret, err := kc.CoreV1().Secrets(namespace).Get(id)
	if err != nil {
		return proj, err
	}

	proj.Name = secret.Name
	proj.Repo.Name = secret.Annotations["projectName"]

	return proj, configureProject(proj, secret.Data, namespace)
}

func def(a []byte, b string) string {
	if len(a) == 0 {
		return b
	}
	return string(a)
}

func configureProject(proj *acid.Project, data map[string][]byte, namespace string) error {
	proj.SharedSecret = def(data["sharedSecret"], "")
	proj.GitHubToken = string(data["githubToken"])

	proj.Kubernetes.Namespace = def(data["namespace"], namespace)
	proj.Kubernetes.VCSSidecar = def(data["vcsSidecar"], DefaultVCSSidecar)

	proj.Repo = acid.Repo{
		Name: def(data["repository"], proj.Name),
		// Note that we have to undo the key escaping.
		SSHKey:   strings.Replace(string(data["sshKey"]), "$", "\n", -1),
		CloneURL: def(data["cloneURL"], ""),
	}

	envVars := map[string]string{}
	if d := data["secrets"]; len(d) > 0 {
		if err := json.Unmarshal(d, &envVars); err != nil {
			return err
		}
	}

	proj.Secrets = envVars
	return nil
}

// ShortSHA returns a 32-char SHA256 digest as a string.
func ShortSHA(input string) string {
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)[0:54]
}