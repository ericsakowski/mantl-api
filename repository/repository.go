package repository

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	consul "github.com/hashicorp/consul/api"
	"path"
	"sort"
	"strconv"
	"strings"
)

const RepositoryRoot = "mantl-install/repository"

type Repository struct {
	Name  string
	Index int
}

type RepositoryCollection []*Repository

func (c RepositoryCollection) Base() *Repository {
	for _, repo := range c {
		if repo.Index == 0 {
			return repo
		}
	}
	return nil
}

func (c RepositoryCollection) Layers() RepositoryCollection {
	var repos RepositoryCollection
	for _, repo := range c {
		if repo.IsBase() {
			continue
		}
		repos = append(repos, repo)
	}
	return repos
}

func (r Repository) PackageIndexKey() string {
	return path.Join(
		RepositoryRoot,
		fmt.Sprintf("%d", r.Index),
		"repo/meta/index.json",
	)
}

func (r Repository) PackagesKey() string {
	return path.Join(
		RepositoryRoot,
		fmt.Sprintf("%d", r.Index),
		"repo/packages",
	)
}

func (r Repository) IsBase() bool {
	return r.Index == 0
}

func Repositories(client *consul.Client) (RepositoryCollection, error) {
	idxs, err := indexes(client)
	if err != nil {
		return nil, err
	}

	var repositories RepositoryCollection
	for _, idx := range idxs {
		name, err := name(client, idx)
		if err != nil {
			log.Warnf("Could not find name for repository %d: %v", idx, err)
			continue
		}

		repositories = append(repositories, &Repository{
			Index: idx,
			Name:  name,
		})
	}

	return repositories, nil
}

func BaseRepository(client *consul.Client) (*Repository, error) {
	kv := client.KV()
	key := path.Join(RepositoryRoot, "0", "name")

	kp, _, err := kv.Get(key, nil)
	if err != nil || kp == nil {
		log.Errorf("Could not retrieve base repository from %s: %v", key, err)
		return nil, err
	}

	return &Repository{Name: string(kp.Value), Index: 0}, nil
}

func Layers(client *consul.Client) (RepositoryCollection, error) {
	repos, err := Repositories(client)
	if err != nil {
		return nil, err
	}
	return repos.Layers(), nil
}

func name(client *consul.Client, idx int) (string, error) {
	kv := client.KV()
	key := path.Join(RepositoryRoot, fmt.Sprintf("%d", idx), "name")
	kp, _, err := kv.Get(key, nil)
	if err != nil || kp == nil {
		log.Errorf("Could not retrieve repository name from %s: %v", key, err)
		return "", err
	}

	return string(kp.Value), nil
}

func indexes(client *consul.Client) ([]int, error) {
	kv := client.KV()

	// retrieves repository indexes like mantl-install/repository/0/
	indexes, _, err := kv.Keys(RepositoryRoot+"/", "/", nil)
	if err != nil {
		return nil, err
	}

	var idxs []int
	for _, key := range indexes {
		parts := strings.Split(strings.TrimSuffix(key, "/"), "/")
		sidx := parts[len(parts)-1]
		idx, err := strconv.Atoi(sidx)
		if err != nil {
			log.Warnf("Unexpected repository index at %s: %v", key, err)
			continue
		}
		idxs = append(idxs, idx)
	}

	sort.Sort(sort.IntSlice(idxs))
	return idxs, nil
}
