package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/jnormington/go-trello"
	"github.com/tj/go-dropbox"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var dateLayout = "2006-01-02T15:04:05.000Z"
var safeFileNameRegexp = regexp.MustCompile(`[^a-zA-Z0-9_.]+`)

// Card holds all the attributes needed for migrating a complete card from Trello to Clubhouse
type Card struct {
	Name        string                   `json:"name"`
	Desc        string                   `json:"desc"`
	Labels      []string                 `json:"labels"`
	DueDate     *time.Time               `json:"due_date"`
	IDCreator   string                   `json:"id_creator"`
	IDOwners    []string                 `json:"id_owners"`
	CreatedAt   *time.Time               `json:"created_at"`
	Comments    []Comment                `json:"comments"`
	Tasks       []Task                   `json:"checklists"`
	Position    float32                  `json:"position"`
	ShortURL    string                   `json:"url"`
	Attachments map[string]TrAttachments `json:"attachments"`
}

// Task builds a basic object based off trello.Task
type Task struct {
	Completed   bool   `json:"completed"`
	Description string `json:"description"`
}

// Comment builds a basic object based off trello.Comment
type Comment struct {
	Text        string
	IDCreator   string
	CreatorName string
	CreatedAt   *time.Time
}

// Attachments for ClubHouse with creator's id
type TrAttachments struct {
	Url       string
	IDCreator string
}

// ProcessCardsForExporting takes *[]trello.Card, *TrelloOptions and builds up a Card
// which consists of calling other functions to make the api calls to Trello
// for the relevant attributes of a card returns *[]Card
func ProcessCardsForExporting(crds *[]trello.Card, opts *TrelloOptions) *[]Card {
	var cards []Card

	for _, card := range *crds {
		var c Card

		c.Name = card.Name
		c.Desc = card.Desc
		c.Labels = getLabelsFlattenFromCard(&card)
		c.DueDate = parseDateOrReturnNil(card.Due)
		c.IDCreator, c.CreatedAt, c.Comments = getCommentsAndCardCreator(&card)
		c.Tasks = getCheckListsForCard(&card)
		c.Position = card.Pos
		c.ShortURL = card.ShortUrl
		c.IDOwners = card.IdMembers

		if opts.ProcessImagesToDropbox {
			c.Attachments = downloadCardAttachmentsUploadToDropbox(&card)
		}
		if opts.ProcessImagesToAWSS3 {
			c.Attachments = downloadCardAttachmentsUploadToAWSS3(&card)
		}

		cards = append(cards, c)
	}

	return &cards
}

func getCommentsAndCardCreator(card *trello.Card) (string, *time.Time, []Comment) {
	var creator string
	var createdAt *time.Time
	var comments []Comment

	actions, err := card.Actions()
	if err != nil {
		fmt.Println("Error: Querying the actions for:", card.Name, "ignoring...", err)
	}

	for _, a := range actions {
		if a.Type == "commentCard" && a.Data.Text != "" {
			c := Comment{
				Text:        a.Data.Text,
				IDCreator:   a.MemberCreator.Id,
				CreatorName: a.MemberCreator.FullName,
				CreatedAt:   parseDateOrReturnNil(a.Date),
			}
			comments = append(comments, c)

		} else if a.Type == "createCard" {
			creator = a.MemberCreator.Id
			createdAt = parseDateOrReturnNil(a.Date)
		}
	}

	return creator, createdAt, comments
}

func getCheckListsForCard(card *trello.Card) []Task {
	var tasks []Task

	checklists, err := card.Checklists()
	if err != nil {
		fmt.Println("Error: Occurred querying checklists for:", card.Name, "ignoring...", err)
	}

	for _, cl := range checklists {
		for _, i := range cl.CheckItems {
			var completed bool
			if i.State == "complete" {
				completed = true
			}

			t := Task{
				Completed:   completed,
				Description: fmt.Sprintf("%s - %s", cl.Name, i.Name),
			}

			tasks = append(tasks, t)
		}
	}

	return tasks
}

func getLabelsFlattenFromCard(card *trello.Card) []string {
	var labels []string

	for _, l := range card.Labels {
		labels = append(labels, l.Name)
	}

	return labels
}

func parseDateOrReturnNil(strDate string) *time.Time {
	d, err := time.Parse(dateLayout, strDate)
	if err != nil {
		//If the date isn't parseable from trello api just return nil
		return nil
	}

	return &d
}

func downloadCardAttachmentsUploadToDropbox(card *trello.Card) map[string]TrAttachments {
	sharedLinks := map[string]TrAttachments{}
	d := dropbox.New(dropbox.NewConfig(dropboxToken))

	attachments, err := card.Attachments()
	if err != nil {
		log.Fatal(err)
	}

	for i, f := range attachments {
		name := safeFileNameRegexp.ReplaceAllString(f.Name, "_")
		path := fmt.Sprintf("/trello/%s/%s/%d%s%s", card.IdList, card.Id, i, "_", name)

		ioReader := downloadTrelloAttachment(&f)
		_, err := d.Files.Upload(&dropbox.UploadInput{
			Path:   path,
			Mode:   dropbox.WriteModeAdd,
			Reader: ioReader,
			Mute:   true,
		})

		ioReader.Close()

		if err != nil {
			fmt.Printf("Error occurred uploading file to dropbox continuing... %s\n", err)
		} else {
			// Must be success created a shared url
			s := dropbox.CreateSharedLinkInput{Path: path}
			out, err := d.Sharing.CreateSharedLink(&s)
			if err != nil {
				fmt.Printf("Error occurred sharing file on dropbox continuing... %s\n", err)
			} else {
				sharedLinks[name] = TrAttachments{out.URL, f.IdMember}
			}
		}
	}

	return sharedLinks
}

func downloadTrelloAttachment(attachment *trello.Attachment) io.ReadCloser {
	resp, err := http.Get(attachment.Url)
	//	defer resp.Body.Close()

	if err != nil {
		log.Fatalf("Error in download Trello attachment %s\n", err)
	}

	return resp.Body
}

func downloadCardAttachmentsUploadToAWSS3(card *trello.Card) map[string]TrAttachments {
	fileIds := map[string]TrAttachments{}
	if len(awsS3Bucket) == 0 {
		log.Fatal("Undefined env variable OPT_AWS_S3_BUCKET.")
	}
	attachments, err := card.Attachments()
	if err != nil {
		log.Fatal(err)
	}

	sess, err := session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		log.Fatal(err)
	}

	uploader := s3manager.NewUploader(sess)
	s3api := s3.New(sess)

	for i, f := range attachments {
		name := safeFileNameRegexp.ReplaceAllString(f.Name, "_")
		folderPath := fmt.Sprintf("/trello/%s/%s/", card.IdList, card.Id)
		path := fmt.Sprintf("%s%d%s%s", folderPath, i, "_", name)

		// check if file already exists
		_, err := s3api.HeadObject(&s3.HeadObjectInput{Bucket: aws.String(awsS3Bucket), Key: aws.String(path)})
		s3apiParams := s3.HeadObjectInput{Bucket: aws.String(awsS3Bucket), Key: aws.String(path)}
		s3keyURL := ""

		if err != nil {
			// uploading file from trello
			resp, err := http.Get(f.Url)
			if err != nil {
				fmt.Printf("Error in download Trello attachment from %s, %s\n", f.Url, err)
				continue
			}

			// Upload the file to S3.
			result, err := uploader.Upload(&s3manager.UploadInput{
				Bucket:      aws.String(awsS3Bucket),
				Key:         aws.String(path),
				ACL:         aws.String("private"),
				Body:        resp.Body,
				ContentType: aws.String(resp.Header.Get("Content-Type")),
			})
			if err != nil {
				fmt.Printf("Error occurred uploading file %s to AWS S3 continuing... %s\n", path, err.Error())
				continue
			} else {
				s3keyURL = aws.StringValue(&result.Location)
			}
			resp.Body.Close()
		} else {
			s3keyURL = fmt.Sprintf("%s/%s%s", s3api.Endpoint, *s3apiParams.Bucket, *s3apiParams.Key)
			fmt.Printf("Skipped uploading file to AWS S3. File exist: %s\n", s3keyURL)
		}

		fileIds[name] = TrAttachments{s3keyURL, f.IdMember}
	}

	return fileIds
}
