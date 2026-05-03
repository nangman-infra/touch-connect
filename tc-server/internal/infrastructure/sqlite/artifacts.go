package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveArtifactVersion(version domain.ArtifactVersion) error {
	body, err := encode(version)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO artifact_versions(artifact_version_ref, message_ref, attempt_ref, body) VALUES(?, ?, ?, ?)`,
		version.ArtifactVersionRef, version.MessageRef, version.AttemptRef, body)
	if err != nil {
		return domain.ErrArtifactExists
	}
	return nil
}

func (s *Store) GetArtifactVersion(artifactVersionRef string) (domain.ArtifactVersion, bool) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM artifact_versions WHERE artifact_version_ref = ?`, artifactVersionRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.ArtifactVersion{}, false
	}
	if err != nil {
		return domain.ArtifactVersion{}, false
	}
	version, err := decode[domain.ArtifactVersion](body)
	return version, err == nil
}
