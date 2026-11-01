package userData

import (
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
)

func Metadata(path *commonUserData.Path) Data {
	data := Data{}
	err, metadatas := path.Metadatas()
	if err != nil {
		return data
	}
	for metadata := range metadatas.Iter() {
		if metadata.Type() == commonUserData.TypeActive {
			data.Path = metadata.Path()
			break
		}
	}
	return data
}
