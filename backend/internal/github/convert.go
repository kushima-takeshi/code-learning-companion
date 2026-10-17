package github

func convertCommits(items []commitResponse) []Commit {
	results := make([]Commit, 0, len(items))

	for _, item := range items {
		result := Commit{
			SHA:        item.SHA,
			Message:    item.Commit.Message,
			AuthorName: item.Commit.Author.Name,
		}

		if item.Author != nil {
			login := item.Author.Login
			result.AuthorLogin = &login
		}

		results = append(results, result)
	}

	return results
}

func convertReviewComments(
	items []reviewCommentResponse,
) []ReviewComment {
	results := make([]ReviewComment, 0, len(items))

	for _, item := range items {
		result := ReviewComment{
			Path:      item.Path,
			Line:      item.Line,
			Body:      item.Body,
			Author:    item.User.Login,
			CommitSHA: item.CommitID,
		}

		results = append(results, result)
	}

	return results
}
