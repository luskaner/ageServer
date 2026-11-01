package userData

import (
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
)

var profiles []Data

func setProfileData(path *commonUserData.Path) bool {
	profiles = make([]Data, 0)
	err, commonProfiles := path.Profiles()
	if err != nil {
		return false
	}
	for entry := range commonProfiles.Iter() {
		if entry.Type() == commonUserData.TypeActive {
			profiles = append(profiles, Data{entry.Path()})
		}
	}
	return true
}

func runProfileMethod(path *commonUserData.Path, mainMethod func(data Data) bool, cleanMethod func(data Data) bool, stopOnFailed bool) bool {
	if !setProfileData(path) {
		return false
	}
	for i := range profiles {
		if !mainMethod(profiles[i]) {
			if !stopOnFailed {
				continue
			}
			for j := i - 1; j >= 0; j-- {
				_ = cleanMethod(profiles[j])
			}
			return false
		}
	}
	return true
}

func backupProfile(data Data) bool {
	return data.Backup()
}

func restoreProfile(data Data) bool {
	return data.Restore()
}

func BackupProfiles(path *commonUserData.Path) bool {
	return runProfileMethod(path, backupProfile, restoreProfile, true)
}

func RestoreProfiles(path *commonUserData.Path, reverseFailed bool) bool {
	return runProfileMethod(path, restoreProfile, backupProfile, reverseFailed)
}
