package deepbrid

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/MunifTanjim/stremthru/core"
)

type ResponseError struct {
	ErrorCode int    `json:"error"`
	Message   string `json:"message,omitempty"`
}

func (e *ResponseError) Error() string {
	ret, _ := json.Marshal(e)
	return string(ret)
}

type ResponseEnvelop interface {
	IsSuccess() bool
	GetError() *ResponseError
}

// --- Generic Response wrappers ---

type BaseResponse struct {
	ErrorCode int    `json:"error"`
	Message   string `json:"message,omitempty"`
}

func (r BaseResponse) IsSuccess() bool {
	return r.ErrorCode == 0
}

func (r BaseResponse) GetError() *ResponseError {
	if r.IsSuccess() {
		return nil
	}
	return &ResponseError{
		ErrorCode: r.ErrorCode,
		Message:   r.Message,
	}
}

type APIResponse[T any] struct {
	Header     http.Header
	StatusCode int
	Data       T
}

func newAPIResponse[T any](res *http.Response, data T) APIResponse[T] {
	apiResponse := APIResponse[T]{
		StatusCode: 503,
		Data:       data,
	}
	if res != nil {
		apiResponse.Header = res.Header
		apiResponse.StatusCode = res.StatusCode
	}
	return apiResponse
}

func extractResponseError(statusCode int, body []byte, v ResponseEnvelop) error {
	if !v.IsSuccess() {
		return v.GetError()
	}
	if statusCode >= http.StatusBadRequest {
		return errors.New(string(body))
	}
	return nil
}

func processResponseBody(res *http.Response, err error, v ResponseEnvelop) error {
	if err != nil {
		return err
	}

	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		return err
	}

	err = core.UnmarshalJSON(res.StatusCode, body, v)
	if err != nil {
		return err
	}

	return extractResponseError(res.StatusCode, body, v)
}

// --- User ---

type GetUserData struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Type           string `json:"type"`
	FidelityPoints int    `json:"fidelity_points"`
	Expiration     string `json:"expiration"`
	MaxDownloads   int    `json:"maxDownloads"`
	MaxConnections int    `json:"maxConnections"`
}

type GetUserResponse struct {
	BaseResponse
	GetUserData
}

func (r GetUserResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type GetUserParams struct {
	Ctx
}

func (c APIClient) GetUser(params *GetUserParams) (APIResponse[GetUserData], error) {
	response := &GetUserResponse{}
	res, err := c.Request("GET", "/user", params, response)
	return newAPIResponse(res, response.GetUserData), err
}

// --- User Stats ---

type GetUserStatsData struct {
	Downloads      int    `json:"downloads"`
	Bandwidth      string `json:"bandwidth"`
	BandwidthBytes int64  `json:"bandwidth_bytes"`
	Torrents       int    `json:"torrents"`
	Remote         int    `json:"remote"`
}

type GetUserStatsResponse struct {
	BaseResponse
	GetUserStatsData
}

func (r GetUserStatsResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type GetUserStatsParams struct {
	Ctx
}

func (c APIClient) GetUserStats(params *GetUserStatsParams) (APIResponse[GetUserStatsData], error) {
	response := &GetUserStatsResponse{}
	res, err := c.Request("GET", "/user/stats", params, response)
	return newAPIResponse(res, response.GetUserStatsData), err
}

// --- Generate Link ---

type GenerateLinkData struct {
	ErrorCode    int    `json:"error"`
	Message      string `json:"message"`
	OriginalLink string `json:"original_link"`
	Hoster       string `json:"hoster"`
	HosterIcon   string `json:"hoster-icon"`
	Filename     string `json:"filename"`
	Link         string `json:"link"`
	Stream       string `json:"stream"`
	Size         string `json:"size"`
}

type GenerateLinkResponse struct {
	GenerateLinkData
}

func (r GenerateLinkResponse) IsSuccess() bool {
	return r.GenerateLinkData.ErrorCode == 0
}

func (r GenerateLinkResponse) GetError() *ResponseError {
	if r.IsSuccess() {
		return nil
	}
	return &ResponseError{ErrorCode: r.GenerateLinkData.ErrorCode, Message: r.GenerateLinkData.Message}
}

type GenerateLinkParams struct {
	Ctx
	Link string `json:"link"`
	Pass string `json:"pass,omitempty"`
}

func (c APIClient) GenerateLink(params *GenerateLinkParams) (APIResponse[GenerateLinkData], error) {
	params.JSON = params
	response := &GenerateLinkResponse{}
	res, err := c.Request("POST", "/generate/link", params, response)
	return newAPIResponse(res, response.GenerateLinkData), err
}

// --- Torrents ---

type AddTorrentData struct {
	ErrorCode int    `json:"error"`
	Message   string `json:"message"`
	Id        string `json:"id"`
}

type AddTorrentResponse struct {
	AddTorrentData
}

func (r AddTorrentResponse) IsSuccess() bool {
	return r.AddTorrentData.ErrorCode == 0
}

func (r AddTorrentResponse) GetError() *ResponseError {
	if r.IsSuccess() {
		return nil
	}
	return &ResponseError{ErrorCode: r.AddTorrentData.ErrorCode, Message: r.AddTorrentData.Message}
}

type AddTorrentParams struct {
	Ctx
	Magnet string `json:"magnet,omitempty"`
}

func (c APIClient) AddTorrent(params *AddTorrentParams) (APIResponse[AddTorrentData], error) {
	form := &url.Values{}
	form.Set("magnet", params.Magnet)
	params.Form = form
	response := &AddTorrentResponse{}
	res, err := c.Request("POST", "/torrents/add", params, response)
	return newAPIResponse(res, response.AddTorrentData), err
}

// --- Torrent Info ---

type TorrentInfoData struct {
	ErrorCode int      `json:"error"`
	Message   string   `json:"message"`
	Id        string   `json:"id"`
	Filename  string   `json:"filename"`
	Progress  int      `json:"progress"`
	Seeders   int      `json:"seeders"`
	Speed     string   `json:"speed"`
	Links     []string `json:"links"`
}

type TorrentInfoResponse struct {
	TorrentInfoData
}

func (r TorrentInfoResponse) IsSuccess() bool {
	return r.TorrentInfoData.ErrorCode == 0
}

func (r TorrentInfoResponse) GetError() *ResponseError {
	if r.IsSuccess() {
		return nil
	}
	return &ResponseError{ErrorCode: r.TorrentInfoData.ErrorCode, Message: r.TorrentInfoData.Message}
}

type GetTorrentInfoParams struct {
	Ctx
	Id string
}

func (c APIClient) GetTorrentInfo(params *GetTorrentInfoParams) (APIResponse[TorrentInfoData], error) {
	query := &url.Values{}
	if params.Id != "" {
		query.Set("id", params.Id)
	}
	params.Query = query
	response := &TorrentInfoResponse{}
	res, err := c.Request("GET", "/torrents/info", params, response)
	return newAPIResponse(res, response.TorrentInfoData), err
}

// --- All Torrents ---

type AllTorrentsData map[string]TorrentInfoData

type AllTorrentsResponse map[string]TorrentInfoData

func (r AllTorrentsResponse) IsSuccess() bool {
	return true
}

func (r AllTorrentsResponse) GetError() *ResponseError {
	return nil
}

type GetAllTorrentsParams struct {
	Ctx
}

func (c APIClient) GetAllTorrents(params *GetAllTorrentsParams) (APIResponse[AllTorrentsData], error) {
	response := &AllTorrentsResponse{}
	res, err := c.Request("GET", "/torrents/info", params, response)
	return newAPIResponse(res, AllTorrentsData(*response)), err
}

// --- Downloads ---

type DownloadDataItem struct {
	Filename string `json:"filename"`
	Size     string `json:"size"`
	Original string `json:"original"`
	Download string `json:"download"`
	Date     string `json:"date"`
}

type DownloadsData struct {
	Success bool               `json:"success"`
	Count   int                `json:"count"`
	Data    []DownloadDataItem `json:"data"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
}

type DownloadsResponse struct {
	DownloadsData
}

func (r DownloadsResponse) IsSuccess() bool {
	return r.DownloadsData.Success
}

func (r DownloadsResponse) GetError() *ResponseError {
	if r.IsSuccess() {
		return nil
	}
	return &ResponseError{ErrorCode: 500, Message: "unknown download error"}
}

type GetDownloadsParams struct {
	Ctx
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

func (c APIClient) GetDownloads(params *GetDownloadsParams) (APIResponse[DownloadsData], error) {
	query := &url.Values{}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	params.Query = query
	response := &DownloadsResponse{}
	res, err := c.Request("GET", "/downloads", params, response)
	return newAPIResponse(res, response.DownloadsData), err
}
