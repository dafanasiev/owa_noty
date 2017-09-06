package EWS2010sp1

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tevino/abool"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"encoding/xml"
	"github.com/ChrisTrenkamp/goxpath"
	"github.com/ChrisTrenkamp/goxpath/tree"
	"github.com/ChrisTrenkamp/goxpath/tree/xmltree"
	"log"
)

type NewMessageEventArgs struct {
	Subject   string
	From      string
	FromEmail string
}

type Client interface {
	SubscribeNewMessages(ctx context.Context, username, password string, cb func(ctx context.Context, err error, eArgs *NewMessageEventArgs)) NewMessagesSubscription
}

type NewMessagesSubscription interface {
	Dispose()
}

type client struct {
	Client
	endpoint string
}

type newMessagesSubscription struct {
	NewMessagesSubscription
	stopped *abool.AtomicBool
}

func NewClient(endpoint string) Client {
	return &client{
		endpoint: endpoint,
	}
}

func (c *client) SubscribeNewMessages(ctx context.Context, username, password string, cb func(ctx context.Context, err error, eArgs *NewMessageEventArgs)) NewMessagesSubscription {
	rv := &newMessagesSubscription{
		stopped: abool.New(),
	}

	newMailEventItemIdXPath := goxpath.MustParse(`/Envelope/Body/m:GetStreamingEventsResponse/m:ResponseMessages/m:GetStreamingEventsResponseMessage/m:Notifications/m:Notification/t:NewMailEvent/t:ItemId`)

	go func() {
		soapPrefix := `<soap:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
				xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
				xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"
				xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">`

		soapSuffix := `</soap:Envelope>`

		subReqTxt := soapPrefix + `<soap:Header>
					<t:RequestServerVersion Version="Exchange2010_SP2" />
				</soap:Header>
				<soap:Body>
					<m:Subscribe>
						<m:StreamingSubscriptionRequest>
							<t:FolderIds><t:DistinguishedFolderId Id="inbox" /></t:FolderIds>
							<t:EventTypes><t:EventType>NewMailEvent</t:EventType></t:EventTypes>
						</m:StreamingSubscriptionRequest>
					</m:Subscribe>
				</soap:Body>` + soapSuffix

		subReq1Tpl := soapPrefix + `
		  <soap:Header>
			<t:RequestServerVersion Version="Exchange2010_SP2" />
		  </soap:Header>
		  <soap:Body>
			<m:GetStreamingEvents>
			  <m:SubscriptionIds>
				<t:SubscriptionId>{0}</t:SubscriptionId>
			  </m:SubscriptionIds>
			  <m:ConnectionTimeout>30</m:ConnectionTimeout>
			</m:GetStreamingEvents>
		  </soap:Body>` + soapSuffix

		defaultNotificationEventArgs := NewMessageEventArgs{}

		for {
			if rv.stopped.IsSet() {
				break
			}

			var subscriptionId string
			client := http.DefaultClient
			mailInfoFetcher := newMailInfoFetcher()

			subReq, err := http.NewRequest("POST", c.endpoint, strings.NewReader(subReqTxt))
			if err != nil {
				e := errors.New(fmt.Sprintf("Error while create NewSubscribe request:%v", err))
				cb(ctx, e, nil)
				continue
			}

			subReq.Header.Add("Content-Type", "text/xml")
			subReq.Header.Add("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/FindFolder")
			subReq.SetBasicAuth(username, password)

			resp, err := client.Do(subReq)
			if err != nil {
				e := errors.New(fmt.Sprintf("Error while POST NewSubscribe request:%v", err))
				cb(ctx, e, nil)
				continue
			}

			respBody, _ := ioutil.ReadAll(resp.Body)
			respBodyStr := string(respBody)

			if strings.Contains(respBodyStr, "<m:ResponseCode>NoError</m:ResponseCode>") {
				subIdxStart := strings.Index(respBodyStr, "<m:SubscriptionId>") + len("<m:SubscriptionId>")
				subIdxEnd := strings.Index(respBodyStr, "</m:SubscriptionId>")

				subscriptionId = respBodyStr[subIdxStart:subIdxEnd]
			} else {
				e := errors.New("Unknown error in NewSubscribe response")
				cb(ctx, e, nil)
				continue
			}

			//---------- subsribe NOW --------------- //

			subReq1Txt := strings.Replace(subReq1Tpl, "{0}", subscriptionId, 1)
			subReq1, err := http.NewRequest("POST", c.endpoint, strings.NewReader(subReq1Txt))
			if err != nil {
				e := errors.New(fmt.Sprintf("Error while create subscribe request:%v with subscriptionId: %s", err, subscriptionId))
				cb(ctx, e, nil)
				continue
			}

			subReq1.Header.Add("Content-Type", "text/xml")
			subReq1.Header.Add("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/FindFolder")
			subReq1.SetBasicAuth(username, password)

			resp, err = client.Do(subReq1)
			if err != nil {
				e := errors.New(fmt.Sprintf("Error while POST subscribe request:%v with subscriptionId: %s", err, subscriptionId))
				cb(ctx, e, nil)
				continue
			}

			scanner := bufio.NewReader(resp.Body)
			scannerNB := newNBByteReader(scanner)

			for {
				envelope, err := readOneEnvelope(scannerNB, rv.stopped)
				if rv.stopped.IsSet() {
					break
				}

				if err != nil {
					e := errors.New(fmt.Sprintf("Error while read event stream:%v for subscriptionId: %s", err, subscriptionId))
					cb(ctx, e, nil)
					break
				}

				if len(envelope) == 0 {
					break //EOF, just reconnect
				}

				if strings.Contains(envelope, "<t:NewMailEvent>") {

					envelopeXml, err := xmltree.ParseXML(bytes.NewBufferString(envelope))
					if err != nil {
						log.Printf("WARN: Fail to parse NewMailEvent envelope: %s. Return default values", err.Error())
						cb(ctx, nil, &defaultNotificationEventArgs)
						return
					}

					newMailEventItemIds, err := newMailEventItemIdXPath.ExecNode(envelopeXml, func(o *goxpath.Opts) {
						o.NS[""] = "http://schemas.xmlsoap.org/soap/envelope/"
						o.NS["m"] = "http://schemas.microsoft.com/exchange/services/2006/messages"
						o.NS["t"] = "http://schemas.microsoft.com/exchange/services/2006/types"
					})
					if err != nil || len(newMailEventItemIds) == 0 {
						cb(ctx, nil, &defaultNotificationEventArgs) //parse failed, but we think that new main present...
						return
					}

					for _, newMailEventItemId := range newMailEventItemIds {
						if rv.stopped.IsSet() {
							break
						}

						item_Id := ""
						item_ChangeKey := ""
						for _, itemIdAttr := range newMailEventItemId.(tree.Elem).GetAttrs() {
							attr := itemIdAttr.GetToken().(xml.Attr)
							switch attr.Name.Local {
							case "Id":
								item_Id = attr.Value
								break
							case "ChangeKey":
								item_ChangeKey = attr.Value
								break
							}

							if item_Id != "" && item_ChangeKey != "" {
								break
							}
						}

						if item_Id != "" && item_ChangeKey != "" {
							info, err := mailInfoFetcher.fetch(c.endpoint, username, password, item_Id, item_ChangeKey)
							if err != nil {
								log.Printf("WARN: Fail to fetch mail info: %s for itemId: %s, itemChangeKey: %s. Return default values", err.Error(), item_Id, item_ChangeKey)
								cb(ctx, nil, &defaultNotificationEventArgs)
							} else {
								cb(ctx, nil, &NewMessageEventArgs{
									FromEmail: info.fromEmail,
									From:      info.from,
									Subject:   info.subject,
								})
							}
						}
					}
				}
			}

			scannerNB.Dispose()
		}
	}()

	return rv
}

func (s *newMessagesSubscription) Dispose() {
	s.stopped.Set()
}

func readOneEnvelope(r *nbByteReader, bStop *abool.AtomicBool) (string, error) {
	stringBuf := make([]byte, 0, 512)

	for {
		if bStop.IsSet() {
			return "", nil
		}

		nByte, bTimeout, err := r.TryReadByte(10 * time.Second)
		if err != nil {
			if err == io.EOF {
				break //END OF STREAM
			}

			return "", err
		}

		if bTimeout {
			continue
		}

		stringBuf = append(stringBuf, nByte)

		if nByte == '>' {
			if bytes.HasSuffix(stringBuf, []byte{'<', '/', 'E', 'n', 'v', 'e', 'l', 'o', 'p', 'e', '>'}) {
				break
			}
		}
	}

	return string(stringBuf), nil
}
