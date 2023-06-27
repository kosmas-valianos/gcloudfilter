# gcloudfilter
Define a lexer and parser to enable filtering of GCP projects locally instead of doing expensive Cloud Resource Manager API calls. That API has a **low quota** therefore it is very easy to end up getting rate limited in case your application has to perform many queries. A typical application would use `SearchProjects()` specifying the filter in the `Query` field of the `SearchProjectsRequest` parameter and do an API call to retrieve the projects that match the `Query`. Instead of spamming API calls with the imminent danger of getting rate limited you can now GET all projects _(`SearchProjects()` with empty `Query` or `ListProjects()` or whatever other way)_ at **every X interval** and use the `FilterProjects()` from this package to filter locally by running the query on the cached projects. In that way the API calls are drastically reduced to 1 per interval instead of 1 per query request. The grammar and syntax are specified in [gcloud topic filters](https://cloud.google.com/sdk/gcloud/reference/topic/filters)

## Installation
```
go get github.com/kosmas-valianos/gcloudfilter
```

## Usage
Check the unit tests of `FilterProjects()`

## Caveats - Not supported yet
1. Parentheses to group expressions like `(labels.color="red" OR parent.id:123.4) OR name:HOWL`
2. Conjunction to have lower precedence than OR
