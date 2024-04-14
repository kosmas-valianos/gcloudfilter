# gcloudfilter
Define a lexer and parser to enable filtering of GCP projects, instances and forwarding rules **locally** instead of doing expensive API calls. Especially the API for the projects has a **low quota** therefore it is very easy to end up getting rate limited in case your application has to perform many queries. A typical application would specify the filter in the `Query`/`Filter` field of the `Request` object parameter and do an API call to retrieve the resources that match that `Query`/`Filter`. Instead of spamming API calls with the imminent danger of getting rate limited you can now request **all** the resources you want at **every X interval** and use the `FilterProjects()`/`FilterInstances()`/`FilterForwardingRules` from this package to filter locally by running the query on the cached resources. In that way the API calls are drastically reduced to a constant 1 per interval instead of 1 per query request! For example an application that has to make 10000 requests it would have to make 10000 API calls but now it will be only 1... The grammar and syntax are specified in [gcloud topic filters](https://cloud.google.com/sdk/gcloud/reference/topic/filters)

## Installation
```
go get github.com/kosmas-valianos/gcloudfilter
```

## Usage/Example
A lot of raw queries can be seen in the unit tests.

The following application downloads and caches all the projects using `SearchProjects()` with 60 seconds update interval. The user can run endless projects' queries using the standard input without worrying about any quota limits as the filtering is happening locally using the `FilterProjects()` on the cached projects.

```golang
type projectsGCP struct {
	resources []*resourcemanagerpb.Project
	err       error
	RWMutex   *sync.RWMutex
}

var projects projectsGCP = projectsGCP{RWMutex: &sync.RWMutex{}}

func updateProjects() {
	projects.RWMutex.Lock()
	projects.err = nil
	defer func() {
		if projects.err != nil {
			fmt.Println(projects.err)
		}
		projects.RWMutex.Unlock()
	}()
	projects.resources = make([]*resourcemanagerpb.Project, 0)

	ctx := context.Background()
	projectsClient, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		projects.err = fmt.Errorf("projects update: failed to create client: %w", err)
		return
	}
	defer projectsClient.Close()

	var req resourcemanagerpb.SearchProjectsRequest
	it := projectsClient.SearchProjects(ctx, &req)
	var sb strings.Builder
	sb.Grow(256)
	sb.WriteString("Projects cached: ")
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			projects.err = fmt.Errorf("projects update: failed to advance iterator: %w", err)
			break
		}
		projects.resources = append(projects.resources, resp)
		sb.WriteString(fmt.Sprintf("%v ", resp.ProjectId))
	}
	fmt.Println(sb.String())
}

func updateProjectsTicker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for t := range ticker.C {
		fmt.Printf("GET all the projects to cache them at %v\n", t.Format(time.DateTime))
		updateProjects()
	}

}

func readInput() {
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Enter GCP project filter:")
		str, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		str = str[:len(str)-1]
		fmt.Printf("Filter query: %v\n", str)
		projects.RWMutex.RLock()
		projectsFiltered, err := gcloudfilter.FilterProjects(projects.resources, str)
		projects.RWMutex.RUnlock()
		if err != nil {
			log.Fatal(fmt.Errorf("applying project filter: %v failed: %w", str, err))
		}
		var sb strings.Builder
		sb.Grow(256)
		sb.WriteString("Projects after filtering: ")
		for _, project := range projectsFiltered {
			sb.WriteString(fmt.Sprintf("%v ", project.ProjectId))
		}
		fmt.Println(sb.String())
	}
}

func main() {
	updateProjects()
	go updateProjectsTicker()
	readInput()
}
```

```
# ./gcloudfilter_example 
Projects cached: corelogic-30-603 appgate-test appgate-dev product-team-222016 
Enter GCP project filter:
id=("appgate-dev" "foo") AND (-labels.boo:* OR labels.envy:*)
Filter query: id=("appgate-dev" "foo") AND (-labels.boo:* OR labels.envy:*)
Projects after filtering: appgate-dev 
Enter GCP project filter:

```

