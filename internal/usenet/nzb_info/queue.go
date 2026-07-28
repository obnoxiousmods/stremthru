package nzb_info

import (
	"github.com/MunifTanjim/stremthru/internal/config"
	"github.com/MunifTanjim/stremthru/internal/job/job_queue"
	"github.com/MunifTanjim/stremthru/internal/util"
)

const JobQueueName = "nzb"

type JobData struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Category  string `json:"category"`
	Password  string `json:"password"`
	User      string `json:"user"`
	Priority  int    `json:"priority"`
	IndexerId int64  `json:"indexer_id,omitempty"`
}

var queue = job_queue.NewPersistentJobQueue(JobQueueName, job_queue.JobQueueConfig[JobData]{
	GetKey: func(item *JobData) string {
		return util.HashNZBFileLink(item.URL)
	},
	Disabled: !config.Feature.HasNewz() || !config.Feature.HasVault(),
})

type JobEntry = job_queue.JobQueueEntry[JobData]

func QueueJob(user, name, url, category string, priority int, password string, indexerId int64) (string, error) {
	err := scheduler.Trigger(JobData{
		Name:      name,
		URL:       url,
		Category:  category,
		Password:  password,
		User:      user,
		Priority:  priority,
		IndexerId: indexerId,
	})
	if err != nil {
		return "", err
	}
	return util.HashNZBFileLink(url), nil
}

func GetAllJob() ([]JobEntry, error) {
	return job_queue.GetEntriesByName[JobData](JobQueueName)
}

func GetJobById(id string) (*JobEntry, error) {
	return job_queue.GetEntryByKey[JobData](JobQueueName, id)
}

func DeleteJob(id string) error {
	return job_queue.DeleteEntries(JobQueueName, []string{id})
}
