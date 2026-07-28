package putio

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
	ErrorId      string `json:"error_id"`
	ErrorMessage string `json:"error_message"`
	ErrorType    string `json:"error_type"`
	ErrorUri     string `json:"error_uri"`
	Status       string `json:"status"`
	StatusCode   int    `json:"status_code"`
}

func (e *ResponseError) Error() string {
	ret, _ := json.Marshal(e)
	return string(ret)
}

type ResponseEnvelop interface {
	IsSuccess() bool
	GetError() *ResponseError
}

type BaseResponse struct {
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
}

func (r BaseResponse) IsSuccess() bool {
	return r.Status == "OK"
}

func (r BaseResponse) GetError() *ResponseError {
	if r.IsSuccess() {
		return nil
	}
	return &ResponseError{
		Status:     r.Status,
		StatusCode: r.StatusCode,
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

// --- Account Info ---

type AccountInfoData struct {
	Username     string `json:"username"`
	Mail         string `json:"mail"`
	PlanExpireAt string `json:"plan_expiration_date"`
	UserId       int    `json:"user_id"`
}

type AccountInfoResponse struct {
	BaseResponse
	Info AccountInfoData `json:"info"`
}

func (r AccountInfoResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type GetAccountInfoParams struct {
	Ctx
}

func (c APIClient) GetAccountInfo(params *GetAccountInfoParams) (APIResponse[AccountInfoData], error) {
	response := &AccountInfoResponse{}
	res, err := c.Request("GET", "/account/info", params, response)
	return newAPIResponse(res, response.Info), err
}

// --- Transfers ---

type TransferData struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	Status        string `json:"status"`
	Downloaded    int64  `json:"downloaded"`
	PercentDone   int    `json:"percent_done"`
	FileId        int    `json:"file_id"`
	CurrentRatio  string `json:"current_ratio"`
	CreatedAt     string `json:"created_at"`
	StatusMessage string `json:"status_message"`
	IsPrivate     bool   `json:"is_private"`
}

type TransferResponse struct {
	BaseResponse
	Transfer TransferData `json:"transfer"`
}

func (r TransferResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type TransferListData struct {
	Transfers []TransferData `json:"transfers"`
}

type TransferListResponse struct {
	BaseResponse
	TransferListData
}

func (r TransferListResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type AddTransferParams struct {
	Ctx
	Url string `json:"url"` // magnet link or torrent URL
}

func (c APIClient) AddTransfer(params *AddTransferParams) (APIResponse[TransferData], error) {
	form := &url.Values{}
	form.Set("url", params.Url)
	params.Form = form
	response := &TransferResponse{}
	res, err := c.Request("POST", "/transfers/add", params, response)
	return newAPIResponse(res, response.Transfer), err
}

type GetTransferParams struct {
	Ctx
	Id int
}

func (c APIClient) GetTransfer(params *GetTransferParams) (APIResponse[TransferData], error) {
	response := &TransferResponse{}
	res, err := c.Request("GET", "/transfers/"+strconv.Itoa(params.Id), params, response)
	return newAPIResponse(res, response.Transfer), err
}

type ListTransfersParams struct {
	Ctx
}

func (c APIClient) ListTransfers(params *ListTransfersParams) (APIResponse[TransferListData], error) {
	response := &TransferListResponse{}
	res, err := c.Request("GET", "/transfers/list", params, response)
	return newAPIResponse(res, response.TransferListData), err
}

type RemoveTransferParams struct {
	Ctx
	Ids []int
}

func (c APIClient) RemoveTransfer(params *RemoveTransferParams) (APIResponse[any], error) {
	form := &url.Values{}
	idsStr := ""
	for i, id := range params.Ids {
		if i > 0 {
			idsStr += ","
		}
		idsStr += strconv.Itoa(id)
	}
	form.Set("transfer_ids", idsStr)
	params.Form = form
	response := &BaseResponse{}
	res, err := c.Request("POST", "/transfers/cancel", params, response)
	return newAPIResponse[any](res, nil), err
}

// --- Files ---

type FileData struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	IsMp4Avail  bool   `json:"is_mp4_available"`
	ParentId    int    `json:"parent_id"`
	IsDir       bool   `json:"is_dir"`
	CreatedAt   string `json:"created_at"`
}

type FileListData struct {
	Files  []FileData `json:"files"`
	Parent FileData   `json:"parent"`
	Total  int        `json:"total"`
}

type FileListResponse struct {
	BaseResponse
	FileListData
}

func (r FileListResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type ListFilesParams struct {
	Ctx
	ParentId int
}

func (c APIClient) ListFiles(params *ListFilesParams) (APIResponse[FileListData], error) {
	query := &url.Values{}
	if params.ParentId > 0 {
		query.Set("parent_id", strconv.Itoa(params.ParentId))
	}
	params.Query = query
	response := &FileListResponse{}
	res, err := c.Request("GET", "/files/list", params, response)
	return newAPIResponse(res, response.FileListData), err
}

type GetFileParams struct {
	Ctx
	Id int
}

func (c APIClient) GetFile(params *GetFileParams) (APIResponse[FileData], error) {
	response := &struct {
		BaseResponse
		File FileData `json:"file"`
	}{}
	res, err := c.Request("GET", "/files/"+strconv.Itoa(params.Id), params, response)
	return newAPIResponse(res, response.File), err
}

// --- Download ---

type DownloadResponse struct {
	BaseResponse
	URL string `json:"url"`
}

func (r DownloadResponse) IsSuccess() bool {
	return r.BaseResponse.IsSuccess()
}

type GetDownloadLinkParams struct {
	Ctx
	Id int
}

func (c APIClient) GetDownloadLink(params *GetDownloadLinkParams) (APIResponse[string], error) {
	response := &DownloadResponse{}
	res, err := c.Request("GET", "/files/"+strconv.Itoa(params.Id)+"/download", params, response)
	return newAPIResponse(res, response.URL), err
}
