package traqchat

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"

	"github.com/antihax/optional"
	traq "github.com/sapphi-red/go-traq"
	traqbot "github.com/traPtitech/traq-bot"
)

type TraqChat struct {
	ID                string // Bot uuid
	UserID            string // Bot user uuid
	AccessToken       string
	VerificationToken string
	Client            *traq.APIClient
	Auth              context.Context
	Handlers          traqbot.EventHandlers
	Matchers          map[*regexp.Regexp]Pattern
	Stamps            map[string]string
}

type Payload struct {
	traqbot.MessageCreatedPayload
}

type Res struct {
	TraqChat
	Payload
}

type ResFunc = func(*Res) error

func newPayload(p traqbot.MessageCreatedPayload) Payload {
	return Payload{p}
}

func newRes(c TraqChat, p Payload) *Res {
	return &Res{
		TraqChat: c,
		Payload:  p,
	}
}

func New(id, uid, at, vt string) *TraqChat {
	client := traq.NewAPIClient(traq.NewConfiguration())
	auth := context.WithValue(context.Background(), traq.ContextAccessToken, at)

	stamps, _, err := client.StampApi.GetStamps(auth, &traq.StampApiGetStampsOpts{})
	if err != nil {
		log.Fatal(err)
	}

	stampsMap := make(map[string]string)
	for _, s := range stamps {
		stampsMap[s.Name] = s.Id
	}

	q := &TraqChat{
		ID:                id,
		UserID:            uid,
		AccessToken:       at,
		VerificationToken: vt,
		Client:            client,
		Auth:              auth,
		Handlers:          traqbot.EventHandlers{},
		Matchers:          map[*regexp.Regexp]Pattern{},
		Stamps:            stampsMap,
	}

	q.Handlers.SetMessageCreatedHandler(func(payload *traqbot.MessageCreatedPayload) {
		for m, p := range q.Matchers {
			if m.MatchString(payload.Message.Text) && p.CanExecute(payload, q.UserID) {
				p.Func(newRes(*q, newPayload(*payload)))
			}
		}
	})

	return q
}

func (q *TraqChat) Hear(re *regexp.Regexp, f ResFunc) error {
	if _, ok := q.Matchers[re]; ok {
		return errors.New("Already Exists")
	}

	q.Matchers[re] = Pattern{
		Func:        f,
		NeedMention: false,
	}

	return nil
}

func (q *TraqChat) Respond(re *regexp.Regexp, f ResFunc) error {
	if _, ok := q.Matchers[re]; ok {
		return errors.New("Already Exists")
	}

	q.Matchers[re] = Pattern{
		Func:        f,
		NeedMention: true,
	}

	return nil
}

func (q *TraqChat) Start() {
	server := traqbot.NewBotServer(q.VerificationToken, q.Handlers)
	log.Fatal(server.ListenAndServe(":80"))
}

func (r *Res) Send(content string) (traq.Message, error) {
	message, _, err := r.Client.MessageApi.PostMessage(r.Auth, r.Message.ChannelID, &traq.MessageApiPostMessageOpts{
		PostMessageRequest: optional.NewInterface(traq.PostMessageRequest{
			Content: content,
			Embed:   true,
		}),
	})
	if err != nil {
		log.Println(fmt.Errorf("failed to send a message: %w", err))

		return traq.Message{}, err
	}

	return message, nil
}

func (r *Res) Reply(content string) (traq.Message, error) {
	message, _, err := r.Client.MessageApi.PostMessage(r.Auth, r.Message.ChannelID, &traq.MessageApiPostMessageOpts{
		PostMessageRequest: optional.NewInterface(traq.PostMessageRequest{
			Content: fmt.Sprintf("@%s %s", r.Message.User.Name, content),
			Embed:   true,
		}),
	})
	if err != nil {
		log.Println(fmt.Errorf("failed to reply a message: %w", err))

		return traq.Message{}, err
	}

	return message, nil
}

func (r *Res) AddStamp(stampName string) error {
	sid, ok := r.Stamps[stampName]
	if !ok {
		return fmt.Errorf("stamp \"%s\" not found", stampName)
	}

	_, err := r.Client.MessageApi.AddMessageStamp(r.Auth, r.Message.ID, sid, &traq.MessageApiAddMessageStampOpts{})
	if err != nil {
		log.Println(fmt.Errorf("failed to add a stamp: %w", err))

		return err
	}

	return err
}
