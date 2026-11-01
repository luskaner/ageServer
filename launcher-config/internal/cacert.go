package internal

import (
	"crypto/x509"
	"fmt"
	"io"
	"os"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	launcherCommonCert "github.com/luskaner/ageServer/common/game/cert"
	"github.com/luskaner/ageServer/common/logger"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
)

type CACert struct {
	launcherCommonCert.CA
}

func NewCACert(gameId string, gamePath string) *CACert {
	if ok, caCert := launcherCommonCert.NewCA(gameId, gamePath); ok {
		return &CACert{caCert}
	}
	return nil
}

func (c *CACert) Backup() (err error) {
	originalPath := c.OriginalPath()
	if _, err = os.Stat(originalPath); err != nil {
		return
	}
	backupPath := c.BackupPath()
	if _, err = os.Stat(backupPath); err == nil {
		return
	}
	commonLogger.Println(commonUi.Step("Opening %s", originalPath))
	originalFile, err := os.Open(originalPath)
	if err != nil {
		return
	}
	defer func(originalFile *os.File) {
		_ = originalFile.Close()
	}(originalFile)
	commonLogger.Println(commonUi.Step("Creating %s", backupPath))
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return
	}
	defer func(backupFile *os.File) {
		_ = backupFile.Close()
	}(backupFile)
	commonLogger.Println(commonUi.Step("Copying data from %s to %s", originalPath, backupPath))
	_, err = io.Copy(backupFile, originalFile)
	if err != nil {
		_ = backupFile.Close()
		_ = os.Remove(backupPath)
		return
	}

	_ = backupFile.Sync()
	return
}

func (c *CACert) Restore() (err error, removedCerts []*x509.Certificate) {
	originalPath := c.OriginalPath()
	if _, err = os.Stat(originalPath); err != nil {
		return
	}
	backupPath := c.BackupPath()
	if _, err = os.Stat(backupPath); err != nil {
		return
	}
	tmpPath := c.TmpPath()
	if _, err = os.Stat(tmpPath); err == nil {
		err = fmt.Errorf("temporary file %s already exists", tmpPath)
		return
	}
	commonLogger.Println(commonUi.Step("Renaming/Moving %s to %s", originalPath, tmpPath))
	err = os.Rename(originalPath, tmpPath)
	if err != nil {
		return
	}
	commonLogger.Println(commonUi.Step("Renaming/Moving %s to %s", backupPath, originalPath))
	err = os.Rename(backupPath, originalPath)
	if err != nil {
		_ = os.Rename(tmpPath, originalPath)
		return
	}
	revert := func() {
		_ = os.Rename(originalPath, backupPath)
		_ = os.Rename(tmpPath, originalPath)
		return
	}
	commonLogger.Println(commonUi.Step("Reading %s certificates", tmpPath))
	backupHashes, backupHashToIndex, backupCerts, err := common.ReadFromFile(tmpPath)
	if err != nil {
		revert()
		return
	}
	commonLogger.Println(commonUi.Step("Reading %s certificates", originalPath))
	originalHashes, _, _, err := common.ReadFromFile(originalPath)
	if err != nil {
		revert()
		return
	}
	commonLogger.Println(commonUi.Step("Deleting %s", tmpPath))
	if err = os.Remove(tmpPath); err != nil {
		revert()
		return
	}
	originalHashesSet := mapset.NewSet[string](originalHashes...)
	backupHashesSet := mapset.NewSet[string](backupHashes...)
	removedHashes := backupHashesSet.Difference(originalHashesSet)
	removedCerts = make([]*x509.Certificate, removedHashes.Cardinality())
	for i, hash := range removedHashes.ToSlice() {
		index, _ := backupHashToIndex[hash]
		removedCerts[i] = backupCerts[index]
	}
	return
}

func (c *CACert) Append(certs []*x509.Certificate) (err error) {
	originalPath := c.OriginalPath()
	if _, err = os.Stat(originalPath); err != nil {
		return
	}
	commonLogger.Println(commonUi.Step("Opening %s", originalPath))
	file, err := os.OpenFile(originalPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	commonLogger.Println(commonUi.Step("Writing certs data"))
	for _, cert := range certs {
		if err = common.WriteAsPem(cert.Raw, file); err != nil {
			return
		}
	}
	return
}
