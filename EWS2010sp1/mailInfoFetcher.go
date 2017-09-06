package EWS2010sp1

import (
	"io/ioutil"
	"net/http"
	"strings"

	"bytes"
	"encoding/xml"
	"github.com/ChrisTrenkamp/goxpath"
	"github.com/ChrisTrenkamp/goxpath/tree/xmltree"
)

type mailInfoFetcher struct {
	http *http.Client

	subjectXPath  goxpath.XPathExec
	fromXPath     goxpath.XPathExec
	getItemReqTpl string
}

type mailInfo struct {
	subject   string
	from      string
	fromEmail string
}

func newMailInfoFetcher() *mailInfoFetcher {
	return &mailInfoFetcher{
		http: &http.Client{},

		subjectXPath: goxpath.MustParse(`/Envelope/Body/m:GetItemResponse/m:ResponseMessages/m:GetItemResponseMessage/m:Items/t:Message/t:Subject`),
		fromXPath:    goxpath.MustParse(`/Envelope/Body/m:GetItemResponse/m:ResponseMessages/m:GetItemResponseMessage/m:Items/t:Message/t:From/t:Mailbox/*`),

		getItemReqTpl: `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:typ="http://schemas.microsoft.com/exchange/services/2006/types" xmlns:mes="http://schemas.microsoft.com/exchange/services/2006/messages">
		   <soapenv:Header>
			  <typ:RequestServerVersion Version="Exchange2010_SP2"/>
			  <typ:MailboxCulture>en-US</typ:MailboxCulture>
		   </soapenv:Header>
		   <soapenv:Body>
			  <mes:GetItem>
				 <mes:ItemShape>
					<typ:BaseShape>Default</typ:BaseShape>
					<typ:IncludeMimeContent>false</typ:IncludeMimeContent>
				 </mes:ItemShape>
				 <mes:ItemIds>
				   <typ:ItemId Id="{0}" ChangeKey="{1}"/>
				 </mes:ItemIds>
			  </mes:GetItem>
		   </soapenv:Body>
		</soapenv:Envelope>`,
	}
}

func (f *mailInfoFetcher) fetch(endpoint string, username string, password string, itemId string, itemChangeKey string) (*mailInfo, error) {
	rv := &mailInfo{}
	getItemReqTplProcessor := strings.NewReplacer("{0}", itemId, "{1}", itemChangeKey)
	getItemReqTxt := getItemReqTplProcessor.Replace(f.getItemReqTpl)
	getItemReq, err := http.NewRequest("POST", endpoint, strings.NewReader(getItemReqTxt))
	if err != nil {
		return nil, err
	}

	getItemReq.Header.Add("Content-Type", "text/xml")
	getItemReq.Header.Add("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/GetItem")
	getItemReq.SetBasicAuth(username, password)

	getItemReqResp, err := f.http.Do(getItemReq)
	if err != nil {
		return nil, err
	}

	getItemReqRespBody, _ := ioutil.ReadAll(getItemReqResp.Body)
	envelope := string(getItemReqRespBody)

	envelopeXml, err := xmltree.ParseXML(bytes.NewBufferString(envelope))
	if err != nil {
		return nil, err
	}

	getByXPathOpts := func(o *goxpath.Opts) {
		o.NS[""] = "http://schemas.xmlsoap.org/soap/envelope/"
		o.NS["m"] = "http://schemas.microsoft.com/exchange/services/2006/messages"
		o.NS["t"] = "http://schemas.microsoft.com/exchange/services/2006/types"
	}

	subjectNode, err := f.subjectXPath.Exec(envelopeXml, getByXPathOpts)
	if err == nil {
		rv.subject = subjectNode.String()
	}

	fromNode, err := f.fromXPath.ExecNode(envelopeXml, getByXPathOpts)
	if err == nil {
		for _, fromC := range fromNode {
			nodeName := fromC.GetToken().(xml.StartElement).Name.Local
			switch nodeName {
			case "Name":
				rv.from = fromC.ResValue()
				break
			case "EmailAddress":
				rv.fromEmail = fromC.ResValue()
				break
			}
		}
	}

	return rv, nil
}
