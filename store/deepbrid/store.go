package deepbrid

import (
	"net/http"
	"path/filepath"
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

const torrentIdPrefix = "st:db:"

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
	c.Name = store.StoreNameDeepBrid

	c.subscriptionStatusCache = cache.NewLRUCache[store.UserSubscriptionStatus](&cache.CacheConfig{
		Name:     "store:deepbrid:subscriptionStatus",
		Lifetime: 5 * time.Minute,
	})

	return c
}

func (c *StoreClient) GetName() store.StoreName {
	return c.Name
}

func (c *StoreClient) GetUser(params *store.GetUserParams) (*store.User, error) {
	start := time.Now()
	res, err := c.client.GetUser(&GetUserParams{
		Ctx: params.Ctx,
	})
	stats.Record(c.Name, "get_user", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}
	data := &store.User{
		Id:                 res.Data.Username,
		Email:              res.Data.Email,
		SubscriptionStatus: store.UserSubscriptionStatusExpired,
	}
	if res.Data.Type == "premium" {
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

func torrentStatusToMagnetStatus(progress int) store.MagnetStatus {
	switch {
	case progress == 100:
		return store.MagnetStatusDownloaded
	case progress > 0 && progress < 100:
		return store.MagnetStatusDownloading
	case progress == 0:
		return store.MagnetStatusQueued
	default:
		return store.MagnetStatusUnknown
	}
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

type LockedFileLink string

const lockedFileLinkPrefix = "stremthru://store/deepbrid/"

func (l LockedFileLink) encodeData(torrentId string, fileIdx int) string {
	return util.Base64Encode(torrentId + ":" + strconv.Itoa(fileIdx))
}

func (l LockedFileLink) decodeData(encoded string) (torrentId string, fileIdx int, err error) {
	decoded, err := util.Base64Decode(encoded)
	if err != nil {
		return "", 0, err
	}
	torrentId, fIdx, found := strings.Cut(decoded, ":")
	if !found {
		return "", 0, err
	}
	fileIdx, err = strconv.Atoi(fIdx)
	if err != nil {
		return "", 0, err
	}
	return torrentId, fileIdx, nil
}

func (l LockedFileLink) create(torrentId string, fileIdx int) string {
	return lockedFileLinkPrefix + l.encodeData(torrentId, fileIdx)
}

func (l LockedFileLink) parse() (torrentId string, fileIdx int, err error) {
	encoded := strings.TrimPrefix(string(l), lockedFileLinkPrefix)
	return l.decodeData(encoded)
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

	// Check if already exists in torrent list
	allTorrents, _ := c.client.GetAllTorrents(&GetAllTorrentsParams{
		Ctx: params.Ctx,
	})
	var existingId string
	for id, t := range allTorrents.Data {
		// DeepBrid doesn't return hash, check filename match as best effort
		if t.Filename == magnet.Name || t.Filename == magnet.RawLink {
			existingId = id
			break
		}
	}

	var t TorrentInfoData
	if existingId != "" {
		tInfo, err := c.client.GetTorrentInfo(&GetTorrentInfoParams{
			Ctx: params.Ctx,
			Id:  existingId,
		})
		if err != nil {
			return nil, err
		}
		t = tInfo.Data
	} else {
		start := time.Now()
		res, err := c.client.AddTorrent(&AddTorrentParams{
			Ctx:    params.Ctx,
			Magnet: magnet.RawLink,
		})
		stats.Record(c.Name, "add_torz", time.Since(start), err != nil)
		if err != nil {
			return nil, err
		}
		// Immediately get torrent info
		start = time.Now()
		tInfo, err := c.client.GetTorrentInfo(&GetTorrentInfoParams{
			Ctx: params.Ctx,
			Id:  res.Data.Id,
		})
		stats.Record(c.Name, "get_torz", time.Since(start), err != nil)
		if err != nil {
			return nil, err
		}
		t = tInfo.Data
	}

	data := &store.AddMagnetData{
		Id:      torrentIdPrefix + t.Id,
		Hash:    magnet.Hash,
		Magnet:  magnet.Link,
		Name:    t.Filename,
		Status:  torrentStatusToMagnetStatus(t.Progress),
		Private: isPrivate,
		Files:   []store.MagnetFile{},
		AddedAt: time.Now(),
	}

	if t.Progress == 100 {
		data.Status = store.MagnetStatusDownloaded
		source := string(c.GetName().Code())
		for idx, link := range t.Links {
			fileName := filepath.Base(link)
			// Clean up filename from query params
			if nameWithoutQuery, _, ok := strings.Cut(fileName, "?"); ok {
				fileName = nameWithoutQuery
			}
			data.Files = append(data.Files, store.MagnetFile{
				Idx:    idx,
				Link:   LockedFileLink("").create(t.Id, idx),
				Name:   fileName,
				Path:   fileName,
				Size:   0,
				Source: source,
			})
		}
	}

	return data, nil
}

func (c *StoreClient) GetMagnet(params *store.GetMagnetParams) (*store.GetMagnetData, error) {
	id := strings.TrimPrefix(params.Id, torrentIdPrefix)
	start := time.Now()
	res, err := c.client.GetTorrentInfo(&GetTorrentInfoParams{
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
		Name:    t.Filename,
		Status:  torrentStatusToMagnetStatus(t.Progress),
		Files:   []store.MagnetFile{},
		AddedAt: time.Now(),
	}

	source := string(c.GetName().Code())
	for idx, link := range t.Links {
		fileName := filepath.Base(link)
		if nameWithoutQuery, _, ok := strings.Cut(fileName, "?"); ok {
			fileName = nameWithoutQuery
		}
		data.Files = append(data.Files, store.MagnetFile{
			Idx:    idx,
			Link:   LockedFileLink("").create(t.Id, idx),
			Name:   fileName,
			Path:   fileName,
			Size:   0,
			Source: source,
		})
	}

	return data, nil
}

func (c *StoreClient) ListMagnets(params *store.ListMagnetsParams) (*store.ListMagnetsData, error) {
	start := time.Now()
	res, err := c.client.GetAllTorrents(&GetAllTorrentsParams{
		Ctx: params.Ctx,
	})
	stats.Record(c.Name, "list_torz", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	data := &store.ListMagnetsData{
		Items:      []store.ListMagnetsDataItem{},
		TotalItems: 0,
	}

	itemsCount := 0
	for id, t := range res.Data {
		itemsCount++
		item := store.ListMagnetsDataItem{
			Id:      torrentIdPrefix + id,
			Name:    t.Filename,
			Status:  torrentStatusToMagnetStatus(t.Progress),
			AddedAt: time.Now(),
		}
		data.Items = append(data.Items, item)
	}
	data.TotalItems = itemsCount

	return data, nil
}

func (c *StoreClient) RemoveMagnet(params *store.RemoveMagnetParams) (*store.RemoveMagnetData, error) {
	// DeepBrid API doesn't have a delete torrent endpoint
	return &store.RemoveMagnetData{
		Id: params.Id,
	}, nil
}

func (c *StoreClient) GenerateLink(params *store.GenerateLinkParams) (*store.GenerateLinkData, error) {
	torrentId, fileIdx, err := LockedFileLink(params.Link).parse()
	if err != nil {
		return nil, err
	}

	// Get torrent info to find the actual link
	tInfo, err := c.client.GetTorrentInfo(&GetTorrentInfoParams{
		Ctx: params.Ctx,
		Id:  torrentId,
	})
	if err != nil {
		return nil, err
	}

	if len(tInfo.Data.Links) <= fileIdx {
		err := UpstreamErrorWithCause(&ResponseError{
			ErrorCode: 404,
			Message:   "file not found",
		})
		err.StatusCode = http.StatusNotFound
		return nil, err
	}

	link := tInfo.Data.Links[fileIdx]

	// The link from torrents/info is a DeepBrid page link, not a direct download
	// We need to generate a download link from it
	start := time.Now()
	genRes, err := c.client.GenerateLink(&GenerateLinkParams{
		Ctx:  params.Ctx,
		Link: link,
	})
	stats.Record(c.Name, "generate_torz_link", time.Since(start), err != nil)
	if err != nil {
		return nil, err
	}

	data := &store.GenerateLinkData{
		Link: genRes.Data.Link,
	}
	return data, nil
}
