package main

import (
	"fmt"
	"github.com/jnormington/go-trello"
	"time"
)

func MarkTrelloCardsAsExportted(cards []trello.Card, tr *TrelloOptions) {
	if !tr.TrelloExportedLable.Exist {
		return
	}
	lblID := tr.TrelloExportedLable.ID

	fmt.Println("Update Cards with Lable: ", trelloExportedLable, ", id:", lblID)
	for _, card := range cards {
		if !checkLableAlreadyExists(card, trelloExportedLable) {
			_, err := card.AddLableByID(lblID)
			if err != nil {
				fmt.Println("Error on updating Card: ", card.Id, err.Error())
			} else {
				fmt.Println("Card", card.ShortUrl, "updated.")
			}
			time.Sleep(7 * time.Millisecond)
		}
	}
}

func checkLableAlreadyExists(c trello.Card, lbl string) bool {
	for _, lable := range c.Labels {
		if lable.Name == lbl {
			return true
		}
	}
	return false
}
