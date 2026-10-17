package github

type ChangedFile struct {
	Filename          string
	Status            string
	Patch             *string
	Content           *string
	ContentSkipReason string
}

type Commit struct {
	SHA         string
	Message     string
	AuthorName  string
	AuthorLogin *string
}

type ReviewComment struct {
	Path      string
	Line      *int
	Body      string
	Author    string
	CommitSHA string
}

type PRSnapshot struct {
	Title          string
	Body           string
	State          string
	Author         string
	HeadSHA        string
	HTMLURL        string
	Diff           string
	Files          []ChangedFile
	Commits        []Commit
	ReviewComments []ReviewComment
}

type prResponse struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	State string `json:"state"`

	User struct {
		Login string `json:"login"`
	} `json:"user"`

	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`

	HTMLURL string `json:"html_url"`
}

type fileResponse struct {
	Filename string  `json:"filename"`
	Status   string  `json:"status"`
	Patch    *string `json:"patch"`
}

type commitResponse struct {
	SHA string `json:"sha"`

	Commit struct {
		Message string `json:"message"`

		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`

	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type reviewCommentResponse struct {
	Path string `json:"path"`
	Line *int   `json:"line"`
	Body string `json:"body"`

	User struct {
		Login string `json:"login"`
	} `json:"user"`

	CommitID string `json:"commit_id"`
}
