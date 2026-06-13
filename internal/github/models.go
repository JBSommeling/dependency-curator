package github

type PRRequest struct {
	Title  string
	Body   string
	Head   string
	Base   string
	Labels []string
	Draft  bool
}

type PullRequest struct {
	Number  int
	HTMLURL string
	State   string
	Head    struct {
		Ref string
		SHA string
	}
}
