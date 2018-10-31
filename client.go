package thecamp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	host      = "https://www.thecamp.or.kr"
	userAgent = "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36"
)

type Client struct {
	hc *http.Client
}

func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &Client{hc: &http.Client{Jar: jar}}, nil
}

func (c *Client) Login(id, pw string) (bool, error) {
	endpoint := "/pcws/common/login.do"
	data := map[string]interface{}{
		"subsType": "1",
		"user-id":  id,
		"user-pwd": pw,
	}

	r, err := c.request(endpoint, data)
	if err != nil {
		return false, err
	}

	return r.Code == 200, nil
}

func (c *Client) Groups() ([]*Group, error) {
	endpoint := "/pcws/troop/group/getMyGroupList.do"
	data := map[string]interface{}{}

	r, err := c.request(endpoint, data)
	if err != nil {
		return nil, err
	}

	t := r.Data.(map[string]interface{})["list2"]

	var res struct {
		Code   int      `json:"result_code"`
		Groups []*Group `json:"my_group"`
	}
	if err := json.Unmarshal([]byte(t.(string)), &res); err != nil {
		return nil, err
	}

	if res.Code != 200 {
		return nil, errors.New("invalid result code")
	}

	for _, g := range res.Groups {
		g.Name = strings.TrimSpace(g.Name)
		g.UnitName = strings.TrimSpace(g.UnitName)
		g.FullName = strings.TrimSpace(g.FullName)
	}

	return res.Groups, nil
}

func (c *Client) TraineeInfo(group *Group) (*TraineeInfo, error) {
	endpoint := "pcws/troop/group/getGroupDetail.do"
	data := map[string]interface{}{"group_id": group.ID}

	r, err := c.request(endpoint, data)
	if err != nil {
		return nil, err
	}

	t := r.Data.(map[string]interface{})["group"]

	var res struct {
		Code        int          `json:"result_code"`
		TraineeInfo *TraineeInfo `json:"trainee_info"`
	}
	if err := json.Unmarshal([]byte(t.(string)), &res); err != nil {
		return nil, err
	}

	if res.Code != 200 {
		return nil, errors.New("invalid result code")
	}

	res.TraineeInfo.Group = group

	return res.TraineeInfo, nil
}

func (c *Client) SendLetter(ti *TraineeInfo, title, content string) (bool, error) {
	endpoint := "/pcws/message/letter/insert.do"
	data := map[string]interface{}{
		"unit_code":    ti.Group.UnitCode,
		"group_id":     ti.Group.ID,
		"trainee_name": ti.Name,
		"birth":        ti.Birthday,
		"relationsip":  ti.Relationship,
		"title":        title,
		"content":      content,
		"fileInfo":     []struct{}{},
	}

	r, err := c.request(endpoint, data)
	if err != nil {
		return false, err
	}

	return r.Code == 200, nil
}

type LettersIterator struct {
	client       *Client
	group        *Group
	lastLetterID *string
	order        sortOrder
	totalCount   *int
	currentCount int
	letters      []*Letter
	idx          int
}

func (c *Client) LettersIterator(group *Group, order sortOrder) *LettersIterator {
	return &LettersIterator{client: c, group: group, order: order}
}

func (li *LettersIterator) Next() (bool, error) {
	stepSize := 30
	if li.totalCount == nil { // initial
		totalCount, letters, err := li.client.letters(li.group, nil, stepSize, li.order)
		if err != nil {
			return false, err
		}

		li.totalCount = &totalCount
		if totalCount == 0 {
			return false, nil
		}

		li.letters = letters
		li.lastLetterID = &letters[len(letters)-1].ID
		li.currentCount = 1
		return true, nil
	} else {
		li.currentCount++
		if li.currentCount > *li.totalCount { // finished
			return false, nil
		}
		li.idx++
		if li.idx > len(li.letters)-1 { // next page
			// TODO: duplicated
			totalCount, letters, err := li.client.letters(li.group, li.lastLetterID, stepSize, li.order)
			if err != nil {
				return false, err
			}

			li.totalCount = &totalCount

			li.letters = letters
			li.lastLetterID = &letters[len(letters)-1].ID
			li.idx = 0
		}
		return true, nil
	}
}

func (li *LettersIterator) Letter() *Letter {
	return li.letters[li.idx]
}

func (c *Client) letters(group *Group, lastLetterID *string, count int, order sortOrder) (int, []*Letter, error) {
	endpoint := "/pcws/message/letter/getList.do"

	orderStr := ""
	switch order {
	default:
		return 0, nil, errors.New("invalid sort order")
	case Ascending:
		orderStr = "ASC"
	case Descending:
		orderStr = "DESC"
	}

	data := map[string]interface{}{
		"unit_code": group.UnitCode,
		"group_id":  group.ID,
		"order":     orderStr,
		"cnt":       count,
	}
	if lastLetterID != nil {
		data["letter_id"] = *lastLetterID
	}

	r, err := c.request(endpoint, data)
	if err != nil {
		return 0, nil, err
	}

	t := r.Data.(map[string]interface{})["list"]

	var res struct {
		Code    int       `json:"result_code"`
		Count   int       `json:"letter_cnt"`
		Letters []*Letter `json:"letter_list"`
	}
	if err := json.Unmarshal([]byte(t.(string)), &res); err != nil {
		return 0, nil, err
	}

	if res.Code != 200 {
		return 0, nil, errors.New("invalid result code")
	}

	return res.Count, res.Letters, nil
}

func (c *Client) request(endpoint string, data interface{}) (*Response, error) {
	u, _ := url.Parse(host)
	u.Path = path.Join(u.Path, endpoint)
	urlString := u.String()

	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", urlString, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil && err != io.EOF {
		return nil, err
	}

	return &r, nil
}

type Response struct {
	Code    int         `json:"resultCode"`
	Message string      `json:"resultMessage"`
	Data    interface{} `json:"resultData"`
}

type Group struct {
	ID          string `json:"group_id"`
	Name        string `json:"group_name"`
	UnitName    string `json:"unit_name"`
	UnitCode    string `json:"unit_code"`
	FullName    string `json:"full_name"`
	EnteredDate string `json:"enter_date"`
}

type TraineeInfo struct {
	Group        *Group `json:"-"`
	Name         string `json:"trainee_name"`
	Birthday     string `json:"birth"`
	Relationship string `json:"relationship"`
}

type sortOrder int

const (
	Ascending sortOrder = iota
	Descending
)

type Letter struct {
	ID        string `json:"letter_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Status    int    `json:"status"`
	TraineeID string `json:"trainee_id"`
	Timestamp int    `json:"create_date"`
}

func (l *Letter) Sent() bool {
	return l.Status == 1
}

func (l *Letter) Date() time.Time {
	loc, _ := time.LoadLocation("Asia/Seoul")
	return time.Unix(int64(l.Timestamp/1000), 0).In(loc)
}
