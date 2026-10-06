package app

import (
	"encoding/base64"
	"errors"
	"path/filepath"

	"uncli/internal/artifacts"
)

// The artifact pane reads artifacts two ways: a version a page recorded
// (by hash, from UNCLI's artifact store), or a file as it is in the
// session's artifacts folder now (one no turn has versioned yet, such as
// files from before versions were kept).

// maxArtifactShown is the biggest artifact the pane loads.
const maxArtifactShown = 10 << 20

// ArtifactRef names an artifact to read: a version's hash, or a path in
// the artifacts folder.
type ArtifactRef struct {
	Hash string `json:"hash,omitempty"`
	Path string `json:"path,omitempty"`
}

// ArtifactContent is an artifact's bytes, base64, for the pane to show.
type ArtifactContent struct {
	Data string `json:"data"`
	Size int    `json:"size"`
}

func (s *Service) artifactsRoot(sessionID string) (string, error) {
	v, err := s.Sessions.View(sessionID)
	if err != nil {
		return "", err
	}
	if p, ok := s.Profiles.Profile(v.ProfileID); !ok || !p.Artifacts {
		return "", errors.New("this session type has no artifacts")
	}
	return filepath.Join(v.Workdir, artifacts.Folder), nil
}

// ArtifactFiles lists what is in a session's artifacts folder now.
func (s *Service) ArtifactFiles(sessionID string) ([]artifacts.File, error) {
	root, err := s.artifactsRoot(sessionID)
	if err != nil {
		return []artifacts.File{}, nil
	}
	return artifacts.List(root)
}

// ReadArtifact returns an artifact's content: a stored version, or the
// file in the session's artifacts folder.
func (s *Service) ReadArtifact(sessionID string, ref ArtifactRef) (ArtifactContent, error) {
	var data []byte
	var err error
	if ref.Hash != "" {
		data, err = s.artifactStore.Read(ref.Hash, maxArtifactShown)
	} else {
		var root string
		if root, err = s.artifactsRoot(sessionID); err == nil {
			data, err = artifacts.ReadLive(root, ref.Path, maxArtifactShown)
		}
	}
	if err != nil {
		return ArtifactContent{}, err
	}
	return ArtifactContent{Data: base64.StdEncoding.EncodeToString(data), Size: len(data)}, nil
}

// ArtifactPath is where an artifact is on disk now, for Open and Show in
// folder; empty if it isn't there.
func (s *Service) ArtifactPath(sessionID, rel string) (string, error) {
	root, err := s.artifactsRoot(sessionID)
	if err != nil {
		return "", err
	}
	return artifacts.Resolve(root, rel)
}
