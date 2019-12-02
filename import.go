package main

import (
	"fmt"
	"strings"
	"time"

	ch "github.com/djthorpe/clubhouse-go"
)

var outputFormat = "%-40s %-17s %s\n"

type exitsCardBehavior int

const (
	SkipExists exitsCardBehavior = iota
	DeleteExists
	AllowDupes
)

var recreateCardsBool = func() exitsCardBehavior {
	switch strings.ToLower(recreateCards) {
	case "1", "true", "yes":
		return DeleteExists
	case "skip":
		return SkipExists
	default:
		return AllowDupes
	}
}()

func getStoryDuplicates(opts *ClubhouseOptions, card Card) []int64 {
	var dupes []int64
	stories, _ := opts.ClubhouseEntry.ListStories(opts.Project.ID)
	for _, story := range stories {
		if story.Name == card.Name {
			dupes = append(dupes, story.ID)
		}
	}
	return dupes
}

// ImportCardsIntoClubhouse takes *[]Card, *ClubhouseOptions and builds a clubhouse Story
// this story from both the card and clubhouse options and creates via the api.
func ImportCardsIntoClubhouse(cards *[]Card, opts *ClubhouseOptions, um *UserMap) {
	fmt.Println("Importing trello cards into Clubhouse...")
	fmt.Printf(outputFormat+"\n", "Trello Card Link", "Import Status", "Error/Story ID")

	for _, c := range *cards {
		c.IDOwners = []string{}
		if recreateCardsBool == DeleteExists {
			for _, id := range getStoryDuplicates(opts, c) {
				err := opts.ClubhouseEntry.DeleteStory(id)
				if err != nil {
					fmt.Printf(outputFormat, c.ShortURL, "Deleted Matching", fmt.Sprintf("Story ID: %d", id))
				}
			}
		}

		if recreateCardsBool == SkipExists {
			if len(getStoryDuplicates(opts, c)) > 0 {
				continue
			}
		}

		//We could use bulk update but lets give the user some prompt feedback
		st, err := opts.ClubhouseEntry.CreateStory(*buildClubhouseStory(&c, opts, um))
		if err != nil {
			fmt.Printf(outputFormat, c.ShortURL, "Failed", err)
			continue
		}

		fmt.Printf(outputFormat, c.ShortURL, "Success", fmt.Sprintf("Story ID: %d", st.ID))
	}
}

func buildLinkFiles(card *Card, opts *ClubhouseOptions, um *UserMap) []int64 {
	var ids []int64

	for k, v := range card.Attachments {
		lf := ch.CreateLinkedFile{
			Name:       k,
			Type:       "url",
			URL:        v.Url,
			UploaderID: um.GetCreator(v.IDCreator),
		}
		r, err := opts.ClubhouseEntry.CreateLinkedFiles(lf)
		fmt.Printf("CARD: %+v\n%+v\n", lf, r)
		if err != nil {
			fmt.Println("Fail to create linked file card name:", card.Name, "link:", v, "Err:", err)
		} else {
			ids = append(ids, r.ID)
		}
	}

	return ids
}

func buildClubhouseStory(card *Card, opts *ClubhouseOptions, um *UserMap) *ch.CreateStory {

	return &ch.CreateStory{
		ProjectID:       opts.Project.ID,
		WorkflowStateID: opts.State.ID,
		RequestedByID:   um.GetCreator(card.IDCreator),
		OwnerIds:        mapOwnersFromTrelloCard(card, um),
		StoryType:       opts.StoryType,
		FollowerIds:     []string{},
		FileIds:         []int64{},

		Name:        card.Name,
		Description: card.Desc,
		Deadline:    card.DueDate,
		CreatedAt:   card.CreatedAt,

		Labels:   *buildLabels(card),
		Tasks:    *buildTasks(card),
		Comments: *buildComments(card, opts.AddCommentWithTrelloLink, um),

		LinkedFileIds: buildLinkFiles(card, opts, um),
	}
}

func mapOwnersFromTrelloCard(c *Card, um *UserMap) []string {
	var owners []string

	for _, o := range c.IDOwners {
		owners = append(owners, um.GetCreator(o))
	}

	return owners
}

func buildComments(card *Card, addCommentWithTrelloLink bool, um *UserMap) *[]ch.CreateComment {
	var comments []ch.CreateComment

	for _, cm := range card.Comments {
		com := ch.CreateComment{
			CreatedAt: *cm.CreatedAt,
			AuthorID:  um.GetCreator(cm.IDCreator),
			Text:      cm.Text,
		}

		comments = append(comments, com)
	}

	if addCommentWithTrelloLink {
		cc := ch.CreateComment{
			CreatedAt: time.Now(),
			Text:      fmt.Sprintf("Card imported from Trello: %s", card.ShortURL),
		}

		comments = append(comments, cc)
	}

	return &comments
}

func buildTasks(card *Card) *[]ch.CreateTask {
	var tasks []ch.CreateTask

	for _, t := range card.Tasks {
		ts := ch.CreateTask{
			Complete:    t.Completed,
			Description: t.Description,
		}

		tasks = append(tasks, ts)
	}

	return &tasks
}

func buildLabels(card *Card) *[]ch.CreateLabel {
	var labels []ch.CreateLabel

	for _, l := range card.Labels {
		labels = append(labels, ch.CreateLabel{Name: l})
	}

	return &labels
}
