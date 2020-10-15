package msteams

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestMSTeams(t *testing.T) {
	unittest.InitGinkgo(t, "Microsoft Teams Util")
}

var _ = Describe("Microsoft Teams", func() {
	g := NewWithT(GinkgoT())

	var msTeamsClient *Client

	var server *httptest.Server

	var (
		tenantID     = "tenantID"
		clientID     = "clientID"
		clientSecret = "clientSecret"
		user         = "user"
		pass         = "pass"
	)

	It("should successfully get access token from Microsoft Graph API", func(done Done) {
		defer close(done)
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(`
{
  "token_type": "Bearer",
  "scope": "profile openid email https://graph.microsoft.com/Directory.ReadWrite.All https://graph.microsoft.com/Group.ReadWrite.All https://graph.microsoft.com/User.ReadWrite.All https://graph.microsoft.com/.default",
  "expires_in": 3599,
  "ext_expires_in": 3599,
  "access_token": "token-123456"
}
`))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
		accessToken, err := msTeamsClient.GetAccessToken()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(accessToken).To(Equal("token-123456"))
	})

	It("should successfully post message with Microsoft Graph API", func(done Done) {
		defer close(done)
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(`
{
  "@odata.context": "https://graph.microsoft.com/beta/$metadata#teams('groupID')/channels('channelID')/messages/$entity",
    "id": "1587552865305",
    "replyToId": null,
    "etag": "1587552865305",
    "messageType": "message",
    "createdDateTime": "2020-04-22T10:54:25.305Z",
    "lastModifiedDateTime": null,
    "deletedDateTime": null,
    "subject": null,
    "summary": null,
    "importance": "normal",
    "locale": "en-us",
    "webUrl": "webURL",
    "policyViolation": null,
    "from": {
        "application": null,
        "device": null,
        "conversation": null,
        "user": {
            "id": "userID",
            "displayName": "samsahai",
            "userIdentityType": "aadUser"
        }
    },
    "body": {
        "contentType": "html",
        "content": "Hi there!"
    },
    "attachments": [],
    "mentions": [],
    "reactions": []
}
`))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
		err := msTeamsClient.PostMessage("groupID", "channelID", "Hi there!", "token-123456")
		g.Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetGroupID", func() {
		It("should successfully get group id from given group id", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`{}`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
			groupID, err := msTeamsClient.GetGroupID("52f44e9b-5cf2-4b77-a6d3-b81e49b7a45c", "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(groupID).To(Equal("52f44e9b-5cf2-4b77-a6d3-b81e49b7a45c"))
		})

		It("should successfully get group id from given group name", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{
 	"@odata.context": "https://graph.microsoft.com/beta/$metadata#teams",
   "@odata.count": 2,
   "value": [
       {
           "id": "5cfa3abf-5b67-46a5-a8ca-b98f98xxxxxx",
           "displayName": "Group 1",
           "description": "",
           "internalId": null,
           "classification": null,
           "specialization": null,
           "visibility": null,
           "webUrl": null,
           "isArchived": false,
           "memberSettings": null,
           "guestSettings": null,
           "messagingSettings": null,
           "funSettings": null,
           "discoverySettings": null
       },
       {
           "id": "52f44e9b-5cf2-4b77-a6d3-b81e49xxxxxx",
           "displayName": "Group 2",
           "description": "",
           "internalId": null,
           "classification": null,
           "specialization": null,
           "visibility": null,
           "webUrl": null,
           "isArchived": false,
           "memberSettings": null,
           "guestSettings": null,
           "messagingSettings": null,
           "funSettings": null,
           "discoverySettings": null
       }
   ]
}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
			groupID, err := msTeamsClient.GetGroupID("Group 2", "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(groupID).To(Equal("52f44e9b-5cf2-4b77-a6d3-b81e49xxxxxx"))
		})

		It("should not get group id due to not found", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{
"@odata.context": "https://graph.microsoft.com/beta/$metadata#teams",
    "@odata.count": 2,
    "value": [
        {
            "id": "5cfa3abf-5b67-46a5-a8ca-b98f98xxxxxx",
            "displayName": "Group 1",
            "description": "",
            "internalId": null,
            "classification": null,
            "specialization": null,
            "visibility": null,
            "webUrl": null,
            "isArchived": false,
            "memberSettings": null,
            "guestSettings": null,
            "messagingSettings": null,
            "funSettings": null,
            "discoverySettings": null
        },
        {
            "id": "52f44e9b-5cf2-4b77-a6d3-b81e49xxxxxx",
            "displayName": "Group 2",
            "description": "",
            "internalId": null,
            "classification": null,
            "specialization": null,
            "visibility": null,
            "webUrl": null,
            "isArchived": false,
            "memberSettings": null,
            "guestSettings": null,
            "messagingSettings": null,
            "funSettings": null,
            "discoverySettings": null
        }
    ]
}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
			_, err := msTeamsClient.GetGroupID("Group 3", "")
			g.Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetChannelID", func() {
		It("should successfully get channel id from given channel id", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{
	"@odata.context": "https://graph.microsoft.com/beta/$metadata#teams('groupID')/channels/$entity",
    "id": "channelID",
    "displayName": "Channel 1",
    "description": null,
    "isFavoriteByDefault": null,
    "email": "",
    "webUrl": "webURL",
    "membershipType": "public"
}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
			channelD, err := msTeamsClient.GetChannelID("groupID", "channelID", "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(channelD).To(Equal("channelID"))
		})

		It("should successfully get channel id from given channel name", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{
  	"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#teams('groupID')/channels",
    "@odata.count": 2,
    "value": [
        {
            "id": "channelID-1",
            "displayName": "Channel 1",
            "description": "",
            "email": "",
            "webUrl": "webURL"
        },
        {
            "id": "channelID-2",
            "displayName": "Channel 2",
            "description": null,
            "email": "",
            "webUrl": "webURL"
        }
    ]
}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
			channelID, err := msTeamsClient.getMatchedChannelID("groupID", "Channel 1", "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(channelID).To(Equal("channelID-1"))
		})
	})

	Specify("Invalid json response", func(done Done) {
		defer close(done)
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(``))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		msTeamsClient = NewClient(tenantID, clientID, clientSecret, user, pass, WithBaseURL(server.URL, server.URL))
		accessToken, err := msTeamsClient.GetAccessToken()
		g.Expect(err).NotTo(BeNil())
		g.Expect(accessToken).To(BeEmpty())
	})
})
