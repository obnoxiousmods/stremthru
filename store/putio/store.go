package putio

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/internal/buddy"
	"github.com/MunifTanjim/stremthru/internal/cache"
	"github.com/MunifTanjim/stremthru/internal/util"
	"github.com/MunifTanjim/stremthru/store"
	"github.com/MunifTanjim/stremthru/store/stats"
)

const magnetIdPrefix = "st:pi:"

type StoreClientConfig struct {
	HTTPClient *http.Client
	UserAgent  string
}

type StoreClient struct {
	Name                    store.StoreName
	client                  *APIClient
	subscriptionStatusCache cache.Cache[store.UserSubscriptionStatus]
}

func NewStoreClient(config *StoreClientConfig) *StoreClient {
	c := &StoreClient{}
	c.client = NewAPIClient(&APIClientConfig{
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
	})
	c.Name = store.StoreNamePutIO

	c.subscriptionStatusCache = cache.NewLRUCache[store.UserSubscriptionStatus](&cache.CacheConfig{
		Name:     "store:putio:subscriptionStatus",
		Lifetime: 5 * time.Minute,
	})

	return c
}

func (c *StoreClient) GetName() store.StoreName {
	return c.Name
}

func (c *StoreClient) GetUser(params *store.GetUserParams) (*store.User, error) {
	start := time.Now()
	res, err := c.client.GetAccountInfo(&GetAccountInfoParams{
		Ctx: params.Ctx,
	})
	stats.Record(c.Name, "get_user", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}
	data := &store.User{
		Id:                 strconv.Itoa(res.Data.UserId),
		Email:              res.Data.Mail,
		SubscriptionStatus: store.UserSubscriptionStatusExpired,
	}
	// Put.io users always have "premium" accounts (it's a paid-only service)
	if res.Data.PlanExpireAt != "" {
		data.SubscriptionStatus = store.UserSubscriptionStatusPremium
	}
	return data, nil
}

func (c *StoreClient) assertValidSubscription(apiKey string) error {
	var status store.UserSubscriptionStatus
	if !c.subscriptionStatusCache.Get(apiKey, &status) {
		params := &store.GetUserParams{}
		params.APIKey = apiKey
		user, err := c.GetUser(params)
		if err != nil {
			return err
		}
		status = user.SubscriptionStatus
		if err := c.subscriptionStatusCache.Add(apiKey, status); err != nil {
			return err
		}
	}
	if status == store.UserSubscriptionStatusPremium {
		return nil
	}
	err := core.NewAPIError("forbidden")
	err.Code = core.ErrorCodeForbidden
	err.StatusCode = http.StatusForbidden
	return err
}

func transferStatusToMagnetStatus(status string, percentDone int) store.MagnetStatus {
	switch status {
	case "COMPLETED":
		return store.MagnetStatusDownloaded
	case "DOWNLOADING":
		return store.MagnetStatusDownloading
	case "IN_QUEUE":
		return store.MagnetStatusQueued
	case "SEEDING":
		if percentDone == 100 {
			return store.MagnetStatusDownloaded
		}
		return store.MagnetStatusDownloading
	case "ERROR":
		return store.MagnetStatusFailed
	default:
		return store.MagnetStatusUnknown
	}
}

type LockedFileLink string

const lockedFileLinkPrefix = "stremthru://store/putio/"

func (l LockedFileLink) encodeData(transferId int, fileId int) string {
	return util.Base64Encode(strconv.Itoa(transferId) + ":" + strconv.Itoa(fileId))
}

func (l LockedFileLink) decodeData(encoded string) (transferId int, fileId int, err error) {
	decoded, err := util.Base64Decode(encoded)
	if err != nil {
		return 0, 0, err
	}
	tId, fId, found := strings.Cut(decoded, ":")
	if !found {
		return 0, 0, err
	}
	transferId, err = strconv.Atoi(tId)
	if err != nil {
		return 0, 0, err
	}
	fileId, err = strconv.Atoi(fId)
	if err != nil {
		return 0, 0, err
	}
	return transferId, fileId, nil
}

func (l LockedFileLink) create(transferId int, fileId int) string {
	return lockedFileLinkPrefix + l.encodeData(transferId, fileId)
}

func (l LockedFileLink) parse() (transferId int, fileId int, err error) {
	encoded := strings.TrimPrefix(string(l), lockedFileLinkPrefix)
	return l.decodeData(encoded)
}

func (c *StoreClient) CheckMagnet(params *store.CheckMagnetParams) (*store.CheckMagnetData, error) {
	if !params.IsTrustedRequest {
		if err := c.assertValidSubscription(params.GetAPIKey(c.client.apiKey)); err != nil {
			return nil, err
		}
	}

	hashes := []string{}
	for _, m := range params.Magnets {
		magnet, err := core.ParseMagnetLink(m)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, magnet.Hash)
	}

	data, err := buddy.CheckMagnet(c, hashes, params.GetAPIKey(c.client.apiKey), params.ClientIP, params.SId, true)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *StoreClient) AddMagnet(params *store.AddMagnetParams) (*store.AddMagnetData, error) {
	var magnet *core.MagnetLink
	var isPrivate bool
	if params.Magnet != "" {
		m, err := core.ParseMagnetLink(params.Magnet)
		if err != nil {
			return nil, err
		}
		magnet = &m
	} else {
		mi, mii, err := params.GetTorrentMeta()
		if err != nil {
			return nil, err
		}
		isPrivate = util.PtrToBool(mii.Private, false)
		m, err := core.ParseMagnetLink(mi.HashInfoBytes().HexString())
		if err != nil {
			return nil, err
		}
		magnet = &m
	}

	start := time.Now()
	res, err := c.client.AddTransfer(&AddTransferParams{
		Ctx: params.Ctx,
		Url: magnet.RawLink,
	})
	stats.Record(c.Name, "add_torz", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	t := res.Data

	data := &store.AddMagnetData{
		Id:      magnetIdPrefix + strconv.Itoa(t.Id),
		Hash:    magnet.Hash,
		Magnet:  magnet.Link,
		Name:    t.Name,
		Size:    t.Size,
		Status:  transferStatusToMagnetStatus(t.Status, t.PercentDone),
		Private: isPrivate,
		Files:   []store.MagnetFile{},
		AddedAt: time.Now(),
	}

	if t.Status == "COMPLETED" && t.FileId > 0 {
		// Get files list for the completed transfer
		filesRes, err := c.client.ListFiles(&ListFilesParams{
			Ctx:      params.Ctx,
			ParentId: t.FileId,
		})
		if err != nil {
			return data, nil // return basic data even if file list fails
		}

		source := string(c.GetName().Code())
		for idx, f := range filesRes.Data.Files {
			if f.IsDir {
				continue
			}
			data.Files = append(data.Files, store.MagnetFile{
				Idx:    idx,
				Link:   LockedFileLink("").create(t.Id, f.Id),
				Name:   f.Name,
				Path:   f.Name,
				Size:   f.Size,
				Source: source,
			})
		}
	}

	return data, nil
}

func (c *StoreClient) GetMagnet(params *store.GetMagnetParams) (*store.GetMagnetData, error) {
	idStr := strings.TrimPrefix(params.Id, magnetIdPrefix)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	res, err := c.client.GetTransfer(&GetTransferParams{
		Ctx: params.Ctx,
		Id:  id,
	})
	stats.Record(c.Name, "get_torz", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	t := res.Data

	data := &store.GetMagnetData{
		Id:      params.Id,
		Name:    t.Name,
		Size:    t.Size,
		Status:  transferStatusToMagnetStatus(t.Status, t.PercentDone),
		Files:   []store.MagnetFile{},
		AddedAt: time.Now(),
	}

	if t.Status == "COMPLETED" && t.FileId > 0 {
		filesRes, err := c.client.ListFiles(&ListFilesParams{
			Ctx:      params.Ctx,
			ParentId: t.FileId,
		})
		if err != nil {
			return data, nil
		}

		source := string(c.GetName().Code())
		for idx, f := range filesRes.Data.Files {
			if f.IsDir {
				continue
			}
			data.Files = append(data.Files, store.MagnetFile{
				Idx:    idx,
				Link:   LockedFileLink("").create(t.Id, f.Id),
				Name:   f.Name,
				Path:   f.Name,
				Size:   f.Size,
				Source: source,
			})
		}
	}

	return data, nil
}

func (c *StoreClient) ListMagnets(params *store.ListMagnetsParams) (*store.ListMagnetsData, error) {
	start := time.Now()
	res, err := c.client.ListTransfers(&ListTransfersParams{
		Ctx: params.Ctx,
	})
	stats.Record(c.Name, "list_torz", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	data := &store.ListMagnetsData{
		Items:      []store.ListMagnetsDataItem{},
		TotalItems: len(res.Data.Transfers),
	}

	for _, t := range res.Data.Transfers {
		item := store.ListMagnetsDataItem{
			Id:      magnetIdPrefix + strconv.Itoa(t.Id),
			Name:    t.Name,
			Size:    t.Size,
			Status:  transferStatusToMagnetStatus(t.Status, t.PercentDone),
			AddedAt: time.Now(),
		}
		data.Items = append(data.Items, item)
	}

	return data, nil
}

func (c *StoreClient) RemoveMagnet(params *store.RemoveMagnetParams) (*store.RemoveMagnetData, error) {
	idStr := strings.TrimPrefix(params.Id, magnetIdPrefix)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	_, err = c.client.RemoveTransfer(&RemoveTransferParams{
		Ctx: params.Ctx,
		Ids: []int{id},
	})
	stats.Record(c.Name, "remove_torz", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	return &store.RemoveMagnetData{
		Id: params.Id,
	}, nil
}

func (c *StoreClient) GenerateLink(params *store.GenerateLinkParams) (*store.GenerateLinkData, error) {
	_, fileId, err := LockedFileLink(params.Link).parse()
	if err != nil {
		return nil, err
	}

	start := time.Now()
	res, err := c.client.GetDownloadLink(&GetDownloadLinkParams{
		Ctx: params.Ctx,
		Id:  fileId,
	})
	stats.Record(c.Name, "generate_torz_link", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	data := &store.GenerateLinkData{
		Link: res.Data,
	}
	return data, nil
}
