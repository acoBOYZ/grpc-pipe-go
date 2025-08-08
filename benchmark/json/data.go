package jsonbench

type Post struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Likes   int32    `json:"likes"`
	Tags    []string `json:"tags"`
}

type Settings struct {
	Theme         string        `json:"theme"`
	Notifications Notifications `json:"notifications"`
}

type Notifications struct {
	Email bool `json:"email"`
	Sms   bool `json:"sms"`
	Push  bool `json:"push"`
}

type Stats struct {
	Posts     int32  `json:"posts"`
	Followers int32  `json:"followers"`
	Following int32  `json:"following"`
	CreatedAt string `json:"createdAt"`
}

type UserProfile struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Bio      string   `json:"bio"`
	Settings Settings `json:"settings"`
	Stats    Stats    `json:"stats"`
	Posts    []Post   `json:"posts"`
}
