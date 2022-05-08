package search

import "fmt"

func SearchByNickname(nickname  string, tag string) string {
	return fmt.Sprintf("%s%s", nickname, tag)
}