package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tonyellard/ess-enn-ess/internal/activity"
	"github.com/tonyellard/ess-enn-ess/internal/message"
	"github.com/tonyellard/ess-enn-ess/internal/subscription"
)

// handleSubscribe handles SNS Subscribe requests
func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request, start time.Time) {
	topicArn := r.FormValue("TopicArn")
	protocol := r.FormValue("Protocol")
	endpoint := r.FormValue("Endpoint")

	if topicArn == "" {
		s.activityLogger.LogError(activity.EventTypeSubscribe, topicArn, "", "topic ARN is required", nil)
		http.Error(w, "TopicArn is required", http.StatusBadRequest)
		return
	}
	if protocol == "" {
		s.activityLogger.LogError(activity.EventTypeSubscribe, topicArn, "", "protocol is required", nil)
		http.Error(w, "Protocol is required", http.StatusBadRequest)
		return
	}
	if endpoint == "" {
		s.activityLogger.LogError(activity.EventTypeSubscribe, topicArn, "", "endpoint is required", nil)
		http.Error(w, "Endpoint is required", http.StatusBadRequest)
		return
	}

	topic, err := s.topicStore.GetTopic(topicArn)
	if err != nil || topic == nil {
		s.activityLogger.LogError(activity.EventTypeSubscribe, topicArn, "", "topic not found", nil)
		http.Error(w, "NotFound: Topic not found", http.StatusNotFound)
		return
	}

	var proto subscription.Protocol
	switch protocol {
	case "http", "https":
		proto = subscription.ProtocolHTTP
	case "sqs":
		proto = subscription.ProtocolSQS
	case "email", "email-json":
		proto = subscription.ProtocolEmail
	case "lambda":
		proto = subscription.ProtocolLambda
	default:
		s.activityLogger.LogError(activity.EventTypeSubscribe, topicArn, "", fmt.Sprintf("unsupported protocol: %s", protocol), nil)
		http.Error(w, fmt.Sprintf("Invalid protocol: %s", protocol), http.StatusBadRequest)
		return
	}

	sub, err := s.subscriptionStore.Create(topicArn, proto, endpoint, s.config.Developer.AutoConfirmSubscriptions)
	if err != nil {
		s.activityLogger.LogError(activity.EventTypeSubscribe, topicArn, "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.topicStore.IncrementSubscriptionCount(topicArn)

	duration := time.Since(start)
	s.activityLogger.Log(activity.EventTypeSubscribe, topicArn, activity.StatusSuccess, map[string]interface{}{
		"subscription_arn": sub.SubscriptionArn,
		"protocol":         protocol,
		"endpoint":         endpoint,
		"status":           sub.Status,
		"duration_ms":      duration.Milliseconds(),
	})

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0"?>
<SubscribeResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
	<SubscribeResult>
		<SubscriptionArn>%s</SubscriptionArn>
	</SubscribeResult>
	<ResponseMetadata>
		<RequestId>%s</RequestId>
	</ResponseMetadata>
</SubscribeResponse>`, sub.SubscriptionArn, generateRequestId())
}

// handleUnsubscribe handles SNS Unsubscribe requests
func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request, start time.Time) {
	subscriptionArn := r.FormValue("SubscriptionArn")
	if subscriptionArn == "" {
		s.activityLogger.LogError(activity.EventTypeUnsubscribe, "", "", "subscription ARN is required", nil)
		http.Error(w, "SubscriptionArn is required", http.StatusBadRequest)
		return
	}

	sub := s.subscriptionStore.GetByArn(subscriptionArn)
	if sub == nil {
		s.activityLogger.LogError(activity.EventTypeUnsubscribe, "", "", "subscription not found", nil)
		http.Error(w, "NotFound: Subscription not found", http.StatusNotFound)
		return
	}

	if err := s.subscriptionStore.Delete(subscriptionArn); err != nil {
		s.activityLogger.LogError(activity.EventTypeUnsubscribe, sub.TopicArn, "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.topicStore.DecrementSubscriptionCount(sub.TopicArn)

	duration := time.Since(start)
	s.activityLogger.Log(activity.EventTypeUnsubscribe, sub.TopicArn, activity.StatusSuccess, map[string]interface{}{
		"subscription_arn": subscriptionArn,
		"duration_ms":      duration.Milliseconds(),
	})

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0"?>
<UnsubscribeResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
	<UnsubscribeResult/>
	<ResponseMetadata>
		<RequestId>%s</RequestId>
	</ResponseMetadata>
</UnsubscribeResponse>`, generateRequestId())
}

// handleListSubscriptionsByTopic handles SNS ListSubscriptionsByTopic requests
func (s *Server) handleListSubscriptionsByTopic(w http.ResponseWriter, r *http.Request, start time.Time) {
	topicArn := r.FormValue("TopicArn")
	if topicArn == "" {
		s.activityLogger.LogError(activity.EventType("list_subscriptions"), topicArn, "", "topic ARN is required", nil)
		http.Error(w, "TopicArn is required", http.StatusBadRequest)
		return
	}

	topic, err := s.topicStore.GetTopic(topicArn)
	if err != nil || topic == nil {
		s.activityLogger.LogError(activity.EventType("list_subscriptions"), topicArn, "", "topic not found", nil)
		http.Error(w, "NotFound: Topic not found", http.StatusNotFound)
		return
	}

	subs := s.subscriptionStore.GetByTopic(topicArn)

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0"?>
<ListSubscriptionsByTopicResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
	<ListSubscriptionsByTopicResult>
		<Subscriptions>`)

	for _, sub := range subs {
		fmt.Fprintf(w, `
			<member>
				<TopicArn>%s</TopicArn>
				<Protocol>%s</Protocol>
				<SubscriptionArn>%s</SubscriptionArn>
				<Owner>%s</Owner>
				<Endpoint>%s</Endpoint>
			</member>`, sub.TopicArn, sub.Protocol, sub.SubscriptionArn, s.config.AWS.AccountId, sub.Endpoint)
	}

	fmt.Fprintf(w, `
		</Subscriptions>
	</ListSubscriptionsByTopicResult>
	<ResponseMetadata>
		<RequestId>%s</RequestId>
	</ResponseMetadata>
</ListSubscriptionsByTopicResponse>`, generateRequestId())

	s.activityLogger.Log(activity.EventType("list_subscriptions"), topicArn, activity.StatusSuccess, map[string]interface{}{
		"subscription_count": len(subs),
		"duration_ms":        time.Since(start).Milliseconds(),
	})
}

// handleGetSubscriptionAttributes handles SNS GetSubscriptionAttributes requests
func (s *Server) handleGetSubscriptionAttributes(w http.ResponseWriter, r *http.Request, start time.Time) {
	subscriptionArn := r.FormValue("SubscriptionArn")
	if subscriptionArn == "" {
		s.activityLogger.LogError(activity.EventType("get_subscription_attributes"), "", "", "subscription ARN is required", nil)
		http.Error(w, "SubscriptionArn is required", http.StatusBadRequest)
		return
	}

	sub := s.subscriptionStore.GetByArn(subscriptionArn)
	if sub == nil {
		s.activityLogger.LogError(activity.EventType("get_subscription_attributes"), "", "", "subscription not found", nil)
		http.Error(w, "NotFound: Subscription not found", http.StatusNotFound)
		return
	}

	attrs, _ := s.subscriptionStore.GetAttributes(subscriptionArn)

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0"?>
<GetSubscriptionAttributesResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
	<GetSubscriptionAttributesResult>
		<Attributes>
			<entry>
				<key>TopicArn</key>
				<value>%s</value>
			</entry>
			<entry>
				<key>Protocol</key>
				<value>%s</value>
			</entry>
			<entry>
				<key>SubscriptionArn</key>
				<value>%s</value>
			</entry>
			<entry>
				<key>Owner</key>
				<value>%s</value>
			</entry>
			<entry>
				<key>Endpoint</key>
				<value>%s</value>
			</entry>
			<entry>
				<key>Status</key>
				<value>%s</value>
			</entry>`, sub.TopicArn, sub.Protocol, sub.SubscriptionArn, s.config.AWS.AccountId, sub.Endpoint, sub.Status)

	for k, v := range attrs {
		fmt.Fprintf(w, `
			<entry>
				<key>%s</key>
				<value>%s</value>
			</entry>`, k, v)
	}

	fmt.Fprintf(w, `
		</Attributes>
	</GetSubscriptionAttributesResult>
	<ResponseMetadata>
		<RequestId>%s</RequestId>
	</ResponseMetadata>
</GetSubscriptionAttributesResponse>`, generateRequestId())

	s.activityLogger.Log(activity.EventType("get_subscription_attributes"), sub.TopicArn, activity.StatusSuccess, map[string]interface{}{
		"subscription_arn": subscriptionArn,
		"duration_ms":      time.Since(start).Milliseconds(),
	})
}

// handleSetSubscriptionAttributes handles SNS SetSubscriptionAttributes requests
func (s *Server) handleSetSubscriptionAttributes(w http.ResponseWriter, r *http.Request, start time.Time) {
	subscriptionArn := r.FormValue("SubscriptionArn")
	attrName := r.FormValue("AttributeName")
	attrValue := r.FormValue("AttributeValue")

	if subscriptionArn == "" {
		s.activityLogger.LogError(activity.EventType("set_subscription_attributes"), "", "", "subscription ARN is required", nil)
		http.Error(w, "SubscriptionArn is required", http.StatusBadRequest)
		return
	}
	if attrName == "" {
		s.activityLogger.LogError(activity.EventType("set_subscription_attributes"), "", "", "attribute name is required", nil)
		http.Error(w, "AttributeName is required", http.StatusBadRequest)
		return
	}

	sub := s.subscriptionStore.GetByArn(subscriptionArn)
	if sub == nil {
		s.activityLogger.LogError(activity.EventType("set_subscription_attributes"), "", "", "subscription not found", nil)
		http.Error(w, "NotFound: Subscription not found", http.StatusNotFound)
		return
	}

	if err := s.subscriptionStore.SetAttribute(subscriptionArn, attrName, attrValue); err != nil {
		s.activityLogger.LogError(activity.EventType("set_subscription_attributes"), sub.TopicArn, "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0"?>
<SetSubscriptionAttributesResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
	<SetSubscriptionAttributesResult/>
	<ResponseMetadata>
		<RequestId>%s</RequestId>
	</ResponseMetadata>
</SetSubscriptionAttributesResponse>`, generateRequestId())

	s.activityLogger.Log(activity.EventType("set_subscription_attributes"), sub.TopicArn, activity.StatusSuccess, map[string]interface{}{
		"subscription_arn": subscriptionArn,
		"attribute_name":   attrName,
		"duration_ms":      time.Since(start).Milliseconds(),
	})
}

// handlePublish handles SNS Publish requests
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request, start time.Time) {
	topicArn := r.FormValue("TopicArn")
	messageText := r.FormValue("Message")
	subject := r.FormValue("Subject")

	if topicArn == "" {
		s.activityLogger.LogError(activity.EventTypePublish, "", "", "topic ARN is required", nil)
		http.Error(w, "TopicArn is required", http.StatusBadRequest)
		return
	}

	if messageText == "" {
		s.activityLogger.LogError(activity.EventTypePublish, topicArn, "", "message is required", nil)
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Verify topic exists
	topic, err := s.topicStore.GetTopic(topicArn)
	if err != nil || topic == nil {
		s.activityLogger.LogError(activity.EventTypePublish, topicArn, "", "topic not found", nil)
		http.Error(w, "NotFound: Topic not found", http.StatusNotFound)
		return
	}

	// Create message
	msg := message.NewMessage(topicArn, subject, messageText, nil)

	// Get all confirmed subscriptions for this topic
	subscriptions := s.subscriptionStore.GetByTopic(topicArn)
	confirmedCount := 0

	// Deliver to each subscription
	ctx := context.Background()
	for _, sub := range subscriptions {
		if sub.Status == subscription.StatusConfirmed {
			confirmedCount++
			// Deliver asynchronously (fire and forget for now)
			go func(sub *subscription.Subscription) {
				if err := s.deliverer.Deliver(ctx, msg, sub); err != nil {
					s.logger.Error("failed to deliver message",
						"message_id", msg.MessageId,
						"subscription_arn", sub.SubscriptionArn,
						"error", err)
				}
			}(sub)
		}
	}

	duration := time.Since(start)
	s.activityLogger.LogWithDuration(activity.EventTypePublish, topicArn, activity.StatusSuccess, duration, map[string]interface{}{
		"message_id":        msg.MessageId,
		"subscriptions":     len(subscriptions),
		"confirmed_subs":    confirmedCount,
		"subject":           subject,
		"message_length":    len(messageText),
	})

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0"?>
<PublishResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/">
	<PublishResult>
		<MessageId>%s</MessageId>
	</PublishResult>
	<ResponseMetadata>
		<RequestId>%s</RequestId>
	</ResponseMetadata>
</PublishResponse>`, msg.MessageId, generateRequestId())
}
