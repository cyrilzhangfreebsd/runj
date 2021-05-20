package jail

import (
	"bytes"
	"fmt"
	"strings"
	"os"
	"path/filepath"
	"text/template"

	"go.sbk.wtf/runj/state"
	"go.sbk.wtf/runj/runtimespec"
)

const (
	confName       = "jail.conf"
	fstabName      = "fstab"
	configTemplate = `{{ .Name }} {
  path = "{{ .Root }}";
  devfs_ruleset = 4;
  mount.devfs;
  mount.fstab = "{{ .Fstab }}";
  persist;
}
`
	fstabTemplate = `{{ .Source }} {{ .Destination }} {{ .Type }} {{ .Options }} 1 1
`
)

func CreateConfig(id, bundle string, ociConfig *runtimespec.Spec) (string, error) {
	config, err := renderConfig(id, bundle, ociConfig)
	if err != nil {
		return "", err
	}
	confPath := ConfPath(id)
	confFile, err := os.OpenFile(confPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("jail: config should not already exist: %w", err)
	}
	defer func() {
		confFile.Close()
		if err != nil {
			os.Remove(confFile.Name())
		}
	}()
	_, err = confFile.Write([]byte(config))
	if err != nil {
		return "", err
	}
	return confFile.Name(), nil
}

func ConfPath(id string) string {
	return filepath.Join(state.Dir(id), confName)
}

func FstabPath(id string) string {
	return filepath.Join(state.Dir(id), fstabName)
}

func renderConfig(id, bundle string, ociConfig *runtimespec.Spec) (string, error) {
	config, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return "", err
	}
	rootPath := ociConfig.Root.Path
	if rootPath[0] != filepath.Separator {
		rootPath = filepath.Join(bundle, rootPath)
	}
	fstab, err := renderFstab(rootPath, ociConfig.Mounts)
	if err != nil {
		return "", err
	}
	fmt.Println(fstab)
	fstabPath := FstabPath(id)
	fstabFile, err := os.OpenFile(fstabPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("jail: fstab should not already exist: %w", err)
	}
	defer func() {
		fstabFile.Close()
		if err != nil {
			os.Remove(fstabFile.Name())
		}
	}()
	_, err = fstabFile.Write([]byte(fstab))
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	config.Execute(&buf, struct {
		Name string
		Root string
		Fstab string
	}{
		Name: id,
		Root: rootPath,
		Fstab: fstabFile.Name(),
	})
	return buf.String(), nil
}

func renderFstab(root string, mounts []runtimespec.Mount) (string, error) {
	fstab, err := template.New("fstab").Parse(fstabTemplate)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	for _, mount := range mounts {
		linebuf := bytes.Buffer{}
		fstab.Execute(&linebuf, struct {
			Source string
			Destination string
			Type string
			Options string
		}{
			Source: mount.Source,
			Destination: filepath.Join(root, mount.Destination),
			Type: mount.Type,
			Options: strings.Join(mount.Options, ","),
		})
		buf.WriteString(linebuf.String())
	}
	return buf.String(), nil
}
