package jobs

import (
	"net/http"
	"strconv"

	"bitbucket.pearson.com/apseng/tensor/api/metadata"
	"bitbucket.pearson.com/apseng/tensor/db"
	"bitbucket.pearson.com/apseng/tensor/models"
	"bitbucket.pearson.com/apseng/tensor/roles"
	"bitbucket.pearson.com/apseng/tensor/util"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

// _CTX_JOB is the key name of the Job Template in gin.Context
const _CTX_JOB = "job"

// _CTX_USER is the key name of the User in gin.Context
const _CTX_USER = "user"

// _CTX_JOB_ID is the key name of http request Job Template ID
const _CTX_JOB_ID = "job_id"

// JobMiddleware is the middleware for job. Which
// takes _CTX_JOB_ID parameter form the request, fetches the Job
// and set it under key _CTX_JOB in gin.Context
func Middleware(c *gin.Context) {
	ID, err := util.GetIdParam(_CTX_JOB_ID, c)

	if err != nil {
		log.Errorln("Error while getting the Job:", err) // log error to the system log
		c.JSON(http.StatusNotFound, models.Error{
			Code:     http.StatusNotFound,
			Messages: []string{"Not Found"},
		})
		c.Abort()
		return
	}

	var job models.Job
	err = db.Jobs().FindId(bson.ObjectIdHex(ID)).One(&job)
	if err != nil {
		log.Errorln("Error while getting the Job:", err) // log error to the system log
		c.JSON(http.StatusNotFound, models.Error{
			Code:     http.StatusNotFound,
			Messages: []string{"Not Found"},
		})
		c.Abort()
		return
	}

	// set Job to the gin.Context
	c.Set(_CTX_JOB, job)
	c.Next() //move to next pending handler
}

// GetJob renders the Job as JSON
// make sure to set this handler next to JobMiddleware handler
func GetJob(c *gin.Context) {
	//get Job set by the middleware
	job := c.MustGet(_CTX_JOB).(models.Job)

	metadata.JobMetadata(&job)

	// send response with JSON rendered data
	c.JSON(http.StatusOK, job)
}

// GetJobs renders the Job as JSON
func GetJobs(c *gin.Context) {
	user := c.MustGet(_CTX_USER).(models.User)

	parser := util.NewQueryParser(c)
	match := bson.M{}
	match = parser.Match([]string{"status", "type", "failed"}, match)
	match = parser.Lookups([]string{"id", "name", "labels"}, match)

	query := db.Jobs().Find(match) // prepare the query

	// set sort value to the query based on request parameters
	if order := parser.OrderBy(); order != "" {
		query.Sort(order)
	}

	var jobs []models.Job

	// new mongodb iterator
	iter := query.Iter()
	// loop through each result and modify for our needs
	var tmpJob models.Job
	// iterate over all and only get valid objects
	for iter.Next(&tmpJob) {
		// if the user doesn't have access to credential
		// skip to next
		if !roles.JobRead(user, tmpJob) {
			continue
		}
		metadata.JobMetadata(&tmpJob)
		// good to go add to list
		jobs = append(jobs, tmpJob)
	}
	if err := iter.Close(); err != nil {
		log.Errorln("Error while retriving Credential data from the db:", err)
		c.JSON(http.StatusInternalServerError, models.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Credential"},
		})
		return
	}

	count := len(jobs)
	pgi := util.NewPagination(c, count)
	//if page is incorrect return 404
	if pgi.HasPage() {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Invalid page " + strconv.Itoa(pgi.Page()) + ": That page contains no results."})
		return
	}
	// send response with JSON rendered data
	c.JSON(http.StatusOK, models.Response{
		Count:    count,
		Next:     pgi.NextPage(),
		Previous: pgi.PreviousPage(),
		Results:  jobs[pgi.Skip():pgi.End()],
	})
}

// CancelInfo to determine if the job can be cancelled.
// The response will include the following field:
// can_cancel: [boolean] Indicates whether this job can be canceled
func CancelInfo(c *gin.Context) {
	//get Job set by the middleware
	// send response with JSON rendered data
	c.JSON(http.StatusOK, gin.H{"can_cancel": false})
}

// Cancel cancels the pending job.
// The response status code will be 202 if successful, or 405 if the job cannot be
// canceled.
func Cancel(c *gin.Context) {
	//get Job set by the middleware
	c.AbortWithStatus(http.StatusMethodNotAllowed)
}

// StdOut returns ANSI standard output of a Job
func StdOut(c *gin.Context) {
	//get Job set by the middleware
	job := c.MustGet(_CTX_JOB).(models.Job)

	c.JSON(http.StatusOK, job.ResultStdout)
}
