package primport

type snapshotKeys struct {
	Body           string
	Diff           string
	Files          string
	Commits        string
	ReviewComments string
}

func buildSnapshotKeys(userID, snapshotID string) snapshotKeys {
	prefix := userID + "/pr-snapshots/" + snapshotID + "/"

	return snapshotKeys{
		Body:           prefix + "body.md",
		Diff:           prefix + "diff.patch",
		Files:          prefix + "files.json",
		Commits:        prefix + "commits.json",
		ReviewComments: prefix + "review-comments.json",
	}
}
