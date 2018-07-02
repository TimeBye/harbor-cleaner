package main

import (
	"github.com/TimeBye/go-harbor"

	"fmt"
	"github.com/golang/glog"
	"time"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"regexp"
	"strings"
	"flag"
	"os"
	"sort"
)

const (
	PROJECTS_PAGE_SIZE     = 50
	REPOSITORIES_PAGE_SIZE = 100
)

var (
	HClient        *harbor.Client
	deletePolicy   DeletePolicy
	policyFilePath string
)

type DeletePolicy struct {
	RegistryUrl         string        `json:"registry_url"`
	UserName            string        `json:"username"`
	Password            string        `json:"password"`
	DryRun              bool          `json:"dry_run"`
	IntervalHour        time.Duration `json:"interval_hour"`
	IgnoreProjects      string        `json:"ignore_projects"`
	MixCount            int           `json:"mix_count"`
	IgnoreProjectsRegex string
	Projects struct {
		DeleteEmpty bool `json:"delete_empty"`
		Include struct {
			Keys      string
			Regex     string
			KeysRegex string
		}
		Exclude struct {
			Keys      string
			Regex     string
			KeysRegex string
		}
	}
	Tags struct {
		Include struct {
			Keys      string
			Regex     string
			KeysRegex string
		}
		Exclude struct {
			Keys      string
			Regex     string
			KeysRegex string
		}
	}
}

type TagSlice []harbor.TagResp

func (tagSlice TagSlice) Len() int {
	return len(tagSlice)
}
func (tagSlice TagSlice) Swap(i, j int) {
	tagSlice[i], tagSlice[j] = tagSlice[j], tagSlice[i]
}
func (tagSlice TagSlice) Less(i, j int) bool {
	return tagSlice[j].Created.Before(tagSlice[i].Created)
}

func checkErrs(errs []error, info string) {
	if errs != nil {
		glog.Exit(func() string {
			for _, v := range errs {
				info = fmt.Sprintf("%v\n%v", info, v)
			}
			return info
		}())
	}
}

func checkErr(err error) {
	if err != nil {
		glog.Exit(err)
	}
}

func getProjects(page int) []harbor.Project {
	projects, resp, errs := HClient.Projects.ListProject(
		&harbor.ListProjectsOptions{
			ListOptions: harbor.ListOptions{
				Page:     page,
				PageSize: PROJECTS_PAGE_SIZE,
			},
		})
	checkErrs(errs, (*resp).Status)
	glog.V(3).Info(func() string {
		s := ""
		for _, v := range projects {
			s = fmt.Sprintf("%v\n%v", v, s)
		}
		return s
	}())
	return projects
}

func getRepositories(projectID int64) []harbor.RepoRecord {
	repositories, resp, errs := HClient.Repositories.ListRepository(
		&harbor.ListRepositoriesOption{
			ListOptions: harbor.ListOptions{
				Page:     1,
				PageSize: REPOSITORIES_PAGE_SIZE,
			},
			ProjectId: projectID,
		})
	checkErrs(errs, (*resp).Status)
	glog.V(3).Info(func() string {
		s := ""
		for _, v := range repositories {
			s = fmt.Sprintf("%v\n%v", v, s)
		}
		return s
	}())
	return repositories
}

func getRepositoryTags(repoName string) []harbor.TagResp {
	tagResp, resp, errs := HClient.Repositories.ListRepositoryTags(repoName)
	checkErrs(errs, (*resp).Status)
	glog.V(3).Info(func() string {
		s := ""
		for _, v := range tagResp {
			s = fmt.Sprintf("%v\n%v", v, s)
		}
		return s
	}())
	return tagResp
}

func generateRegexByKeys(keys string) string {
	if strings.HasPrefix(keys, ",") {
		keys = keys[1:]
	}
	if strings.HasSuffix(keys, ",") {
		keys = keys[:len(keys)-1]
	}
	if len(keys) == 0 || len(strings.Replace(keys, ",", "", -1)) == 0 {
		return ":"
	}
	return fmt.Sprintf(".*%s.*", strings.Replace(keys, ",", ".*|.*", -1))
}

func needDeleteTag(tag harbor.TagResp, count *int) bool {
	baseTime, _ := time.ParseDuration("1h")
	if time.Now().Sub(tag.Created) < deletePolicy.IntervalHour*baseTime || *count < 10 {
		*count += 1
		return false
	}
	match, err := regexp.MatchString(deletePolicy.Tags.Exclude.Regex, tag.Name)
	checkErr(err)
	if match {
		return false
	}
	match, err = regexp.MatchString(deletePolicy.Tags.Exclude.KeysRegex, tag.Name)
	checkErr(err)
	if match {
		return false
	}
	match, err = regexp.MatchString(deletePolicy.Tags.Include.Regex, tag.Name)
	checkErr(err)
	if match {
		glog.V(2).Infof("%s match Tags.Regex: %s", tag.Name, deletePolicy.Tags.Include.Regex)
		return true
	}
	match, err = regexp.MatchString(deletePolicy.Tags.Include.KeysRegex, tag.Name)
	checkErr(err)
	if match {
		glog.V(2).Infof("%s match Tags.KeysRegex: %s", tag.Name, deletePolicy.Tags.Include.KeysRegex)
		return true
	}
	return false
}

func needDeleteProject(project harbor.Project) bool {
	match, err := regexp.MatchString(deletePolicy.Projects.Exclude.Regex, project.Name)
	checkErr(err)
	if match {
		return false
	}
	match, err = regexp.MatchString(deletePolicy.Projects.Exclude.KeysRegex, project.Name)
	checkErr(err)
	if match {
		return false
	}
	match, err = regexp.MatchString(deletePolicy.Projects.Include.Regex, project.Name)
	checkErr(err)
	if match {
		glog.V(2).Infof("%s match Projects.Regex: %s", project.Name, deletePolicy.Projects.Include.Regex)
		return true
	}
	match, err = regexp.MatchString(deletePolicy.Projects.Include.KeysRegex, project.Name)
	checkErr(err)
	if match {
		glog.V(2).Infof("%s match Projects.KeysRegex: %s", project.Name, deletePolicy.Projects.Include.KeysRegex)
		return true
	}
	if project.RepoCount == 0 && deletePolicy.Projects.DeleteEmpty {
		glog.V(2).Infof("Empty Project %s", project.Name)
		return true
	}
	return false
}

func doDelete(statisticMap harbor.StatisticMap) {
	pageCount := statisticMap.TotalProjectCount/PROJECTS_PAGE_SIZE + 1
	for i := pageCount; i != 0; i-- {
		projects := getProjects(i)
		for _, project := range projects {
			match, err := regexp.MatchString(deletePolicy.IgnoreProjectsRegex, project.Name)
			checkErr(err)
			if match {
				continue
			}
			repositories := getRepositories(project.ProjectID)
			for _, repository := range repositories {
				tags := getRepositoryTags(repository.Name)
				sort.Sort(TagSlice(tags))
				var count = 0
				for _, tag := range tags {
					if needDeleteTag(tag, &count) {
						glog.Infof("Untagged: %s:%s", repository.Name, tag.Name)
						if !deletePolicy.DryRun {
							resp, errs := HClient.Repositories.DeleteRepositoryTag(repository.Name, tag.Name)
							if errs != nil {
								glog.Error((*resp).Status)
							}
						}
					}
				}
			}
			if needDeleteProject(project) {
				glog.Infof("Delete Project: %s", project.Name)
				if !deletePolicy.DryRun {
					resp, errs := HClient.Projects.DeleteProject(project.ProjectID)
					if errs != nil {
						glog.Error((*resp).Status)
					}
				}
			}
		}
	}
}

func readDeletePolicy() {
	data, err := ioutil.ReadFile(policyFilePath)
	checkErr(err)
	deletePolicy.DryRun = true
	deletePolicy.IntervalHour = 72
	deletePolicy.MixCount = 10
	err = yaml.Unmarshal(data, &deletePolicy)
	checkErr(err)
	deletePolicy.Projects.Include.KeysRegex = generateRegexByKeys(deletePolicy.Projects.Include.Keys)
	deletePolicy.Projects.Exclude.KeysRegex = generateRegexByKeys(deletePolicy.Projects.Exclude.Keys)
	deletePolicy.Tags.Include.KeysRegex = generateRegexByKeys(deletePolicy.Tags.Include.Keys)
	deletePolicy.Tags.Exclude.KeysRegex = generateRegexByKeys(deletePolicy.Tags.Exclude.Keys)
	deletePolicy.IgnoreProjectsRegex = strings.Replace(deletePolicy.IgnoreProjects, ",", "|", -1)
	if len(deletePolicy.IgnoreProjectsRegex) == 0 {
		deletePolicy.IgnoreProjectsRegex = ":"
	}
	if len(deletePolicy.Projects.Include.Regex) == 0 {
		deletePolicy.Projects.Include.Regex = ":"
	}
	if len(deletePolicy.Projects.Include.KeysRegex) == 0 {
		deletePolicy.Projects.Include.KeysRegex = ":"
	}
	if len(deletePolicy.Projects.Exclude.Regex) == 0 {
		deletePolicy.Projects.Exclude.Regex = ":"
	}
	if len(deletePolicy.Projects.Exclude.KeysRegex) == 0 {
		deletePolicy.Projects.Exclude.KeysRegex = ":"
	}
	if len(deletePolicy.Tags.Include.Regex) == 0 {
		deletePolicy.Tags.Include.Regex = ":"
	}
	if len(deletePolicy.Tags.Include.KeysRegex) == 0 {
		deletePolicy.Tags.Include.KeysRegex = ":"
	}
	if len(deletePolicy.Tags.Exclude.Regex) == 0 {
		deletePolicy.Tags.Exclude.Regex = ":"
	}
	if len(deletePolicy.Tags.Exclude.KeysRegex) == 0 {
		deletePolicy.Tags.Exclude.KeysRegex = ":"
	}
}

func init() {
	flag.Set("logtostderr", "true")
	if len(os.Getenv("LOG_LEVEL")) != 0 {
		logLevel := os.Getenv("LOG_LEVEL")
		flag.Set("v", logLevel)
	}
	flag.StringVar(&policyFilePath, "f", "delete_policy.yml", "Filename, directory, or URL to files that delete policy.")
	flag.Parse()
}

func main() {
	readDeletePolicy()
	HClient = harbor.NewClient(nil, deletePolicy.RegistryUrl, deletePolicy.UserName, deletePolicy.Password)
	statisticMap, resp, errs := HClient.GetStatistics()
	checkErrs(errs, (*resp).Status)
	doDelete(statisticMap)
}
