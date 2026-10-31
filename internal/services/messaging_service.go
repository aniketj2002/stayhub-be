package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/messaging"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/storage"
)

type MessagingService struct {
	db              *db.DB
	messagingRepo   interfaces.MessagingRepository
	reservationRepo interfaces.ReservationRepository
	listingRepo     interfaces.ListingRepository
	listingHostRepo interfaces.ListingHostRepository
	userRepo        interfaces.UserRepository
	mediaRepo       interfaces.MediaRepository
	storage         *storage.Storage
	clock           clock.Clock
	broadcaster     interfaces.MessageBroadcaster
}

func (s *MessagingService) SetMediaDependencies(mediaRepo interfaces.MediaRepository, store *storage.Storage) {
	s.mediaRepo = mediaRepo
	s.storage = store
}

func (s *MessagingService) SetBroadcaster(b interfaces.MessageBroadcaster) {
	s.broadcaster = b
}

func NewMessagingService(
	database *db.DB,
	messagingRepo interfaces.MessagingRepository,
	reservationRepo interfaces.ReservationRepository,
	listingRepo interfaces.ListingRepository,
	listingHostRepo interfaces.ListingHostRepository,
	userRepo interfaces.UserRepository,
	clk clock.Clock,
) interfaces.MessagingService {
	return &MessagingService{
		db:              database,
		messagingRepo:   messagingRepo,
		reservationRepo: reservationRepo,
		listingRepo:     listingRepo,
		listingHostRepo: listingHostRepo,
		userRepo:        userRepo,
		clock:           clk,
	}
}

func (s *MessagingService) GetOrCreateConversation(ctx context.Context, reservationID uuid.UUID) (*dtos.ConversationResponse, error) {
	res, err := s.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	conv, err := s.messagingRepo.GetOrCreateConversation(ctx, res.ID, res.ListingID, res.GuestID)
	if err != nil {
		return nil, err
	}

	return s.buildConversationResponse(ctx, *conv, res.GuestID)
}

func (s *MessagingService) ListConversations(ctx context.Context, userID uuid.UUID) ([]dtos.ConversationResponse, error) {
	convs, err := s.messagingRepo.ListConversationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.ConversationResponse, 0, len(convs))
	for _, c := range convs {
		resp, err := s.buildConversationResponse(ctx, c, userID)
		if err == nil && resp != nil {
			out = append(out, *resp)
		}
	}
	return out, nil
}

func (s *MessagingService) GetConversation(ctx context.Context, userID, conversationID uuid.UUID) (*dtos.ConversationResponse, error) {
	conv, err := s.messagingRepo.FindConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if err := s.authorizeParticipant(ctx, userID, *conv); err != nil {
		return nil, err
	}

	return s.buildConversationResponse(ctx, *conv, userID)
}

func (s *MessagingService) ListMessages(
	ctx context.Context,
	userID, conversationID uuid.UUID,
	before *time.Time,
	limit int,
) (*dtos.MessageListResponse, error) {
	conv, err := s.messagingRepo.FindConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if err := s.authorizeParticipant(ctx, userID, *conv); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Fetch limit + 1 to determine has_more
	messages, err := s.messagingRepo.ListMessagesByConversation(ctx, conversationID, before, limit+1)
	if err != nil {
		return nil, err
	}

	hasMore := false
	if len(messages) > limit {
		hasMore = true
		messages = messages[:limit]
	}

	var nextCursor *string
	if hasMore && len(messages) > 0 {
		cursorStr := messages[len(messages)-1].CreatedAt.Format(time.RFC3339Nano)
		nextCursor = &cursorStr
	}

	msgResponses := make([]dtos.MessageResponse, 0, len(messages))
	for _, m := range messages {
		msgResponses = append(msgResponses, s.toMessageResponse(ctx, m, nil))
	}

	return &dtos.MessageListResponse{
		Messages:   msgResponses,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (s *MessagingService) SendMessage(
	ctx context.Context,
	senderID, conversationID uuid.UUID,
	in dtos.SendMessageInput,
) (*dtos.MessageResponse, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" && len(in.MediaIDs) == 0 {
		return nil, fmt.Errorf("%w: message body cannot be empty", apperrors.ErrValidation)
	}

	conv, err := s.messagingRepo.FindConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if err := s.authorizeParticipant(ctx, senderID, *conv); err != nil {
		return nil, err
	}

	// Run off-platform contact/payment detector
	hasFlag, warningMsg := messaging.DetectOffPlatform(body)
	flags := []string{}
	if hasFlag {
		flags = append(flags, messaging.OffPlatformFlag)
	}

	mediaIDs := in.MediaIDs
	if mediaIDs == nil {
		mediaIDs = []uuid.UUID{}
	}

	now := s.clock.Now()
	msg, err := s.messagingRepo.InsertMessage(ctx, models.Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           body,
		MediaIDs:       mediaIDs,
		Flags:          flags,
		CreatedAt:      now,
	})
	if err != nil {
		return nil, err
	}

	// Mark conversation read for the sender
	_ = s.messagingRepo.MarkConversationRead(ctx, conversationID, senderID, now)

	var warn *string
	if warningMsg != "" {
		warn = &warningMsg
	}

	resp := s.toMessageResponse(ctx, *msg, warn)

	if s.broadcaster != nil {
		participantIDs, _ := s.GetConversationParticipantIDs(ctx, conversationID)
		s.broadcaster.BroadcastNewMessage(ctx, conversationID, resp, participantIDs)
	}

	return &resp, nil
}

func (s *MessagingService) MarkRead(ctx context.Context, userID, conversationID uuid.UUID) error {
	conv, err := s.messagingRepo.FindConversationByID(ctx, conversationID)
	if err != nil {
		return err
	}

	if err := s.authorizeParticipant(ctx, userID, *conv); err != nil {
		return err
	}

	if err := s.messagingRepo.MarkConversationRead(ctx, conversationID, userID, s.clock.Now()); err != nil {
		return err
	}

	if s.broadcaster != nil {
		participantIDs, _ := s.GetConversationParticipantIDs(ctx, conversationID)
		s.broadcaster.BroadcastConversationRead(ctx, conversationID, userID, participantIDs)
	}

	return nil
}

func (s *MessagingService) GetConversationParticipantIDs(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	conv, err := s.messagingRepo.FindConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uuid.UUID]bool)
	var participants []uuid.UUID

	add := func(id uuid.UUID) {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			participants = append(participants, id)
		}
	}

	add(conv.GuestID)

	if l, err := s.listingRepo.FindByID(ctx, conv.ListingID); err == nil && l != nil {
		add(l.HostID)
	}

	if hosts, err := s.listingHostRepo.ListByListing(ctx, conv.ListingID); err == nil {
		for _, h := range hosts {
			if h.Role == "owner" || h.CanMessageGuests {
				add(h.UserID)
			}
		}
	}

	return participants, nil
}

func (s *MessagingService) authorizeParticipant(ctx context.Context, userID uuid.UUID, conv models.Conversation) error {
	if conv.GuestID == userID {
		return nil
	}

	if l, err := s.listingRepo.FindByID(ctx, conv.ListingID); err == nil && l != nil && l.HostID == userID {
		return nil
	}

	// Check if user is host or co-host with can_message_guests
	lh, err := s.listingHostRepo.FindByListingAndUser(ctx, conv.ListingID, userID)
	if err == nil && lh != nil {
		if lh.Role == "owner" || lh.CanMessageGuests {
			return nil
		}
	}

	return apperrors.ErrForbidden
}

func (s *MessagingService) resolveMediaURLs(ctx context.Context, ids []uuid.UUID) []string {
	if len(ids) == 0 {
		return []string{}
	}
	urls := make([]string, 0, len(ids))
	if s.mediaRepo != nil && s.storage != nil {
		if items, err := s.mediaRepo.FindByIDs(ctx, ids); err == nil {
			byID := make(map[uuid.UUID]string, len(items))
			for _, item := range items {
				byID[item.ID] = s.storage.PublicURL(item.S3Key)
			}
			for _, id := range ids {
				if u, ok := byID[id]; ok && u != "" {
					urls = append(urls, u)
				} else {
					urls = append(urls, fmt.Sprintf("/api/v1/media/%s/raw", id.String()))
				}
			}
			return urls
		}
	}
	for _, id := range ids {
		urls = append(urls, fmt.Sprintf("/api/v1/media/%s/raw", id.String()))
	}
	return urls
}

func (s *MessagingService) toMessageResponse(ctx context.Context, m models.Message, warn *string) dtos.MessageResponse {
	mediaIDs := m.MediaIDs
	if mediaIDs == nil {
		mediaIDs = []uuid.UUID{}
	}
	flags := m.Flags
	if flags == nil {
		flags = []string{}
	}
	return dtos.MessageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderID:       m.SenderID,
		Body:           m.Body,
		MediaIDs:       mediaIDs,
		MediaURLs:      s.resolveMediaURLs(ctx, mediaIDs),
		Flags:          flags,
		Warning:        warn,
		CreatedAt:      m.CreatedAt,
	}
}

func (s *MessagingService) buildConversationResponse(
	ctx context.Context,
	c models.Conversation,
	currentUserID uuid.UUID,
) (*dtos.ConversationResponse, error) {
	listingTitle := "Listing"
	listingCity := ""
	listingCoverURL := ""
	var hostID uuid.UUID
	hostName := "Host"

	if l, err := s.listingRepo.FindByID(ctx, c.ListingID); err == nil && l != nil {
		hostID = l.HostID
		if host, err := s.userRepo.FindByID(ctx, l.HostID); err == nil && host != nil && host.FullName != "" {
			hostName = host.FullName
		}
		versionID := l.LiveVersionID
		if versionID == nil {
			versionID = l.WorkingVersionID
		}
		if versionID != nil {
			if ver, err := s.listingRepo.FindVersionByID(ctx, *versionID); err == nil && ver != nil {
				if ver.Title != nil && *ver.Title != "" {
					listingTitle = *ver.Title
				}
				if ver.City != nil {
					listingCity = *ver.City
				}
				if len(ver.MediaIDs) > 0 {
					urls := s.resolveMediaURLs(ctx, ver.MediaIDs[:1])
					if len(urls) > 0 {
						listingCoverURL = urls[0]
					}
				}
			}
		}
	}

	guestName := "Guest"
	if guest, err := s.userRepo.FindByID(ctx, c.GuestID); err == nil && guest != nil && guest.FullName != "" {
		guestName = guest.FullName
	}

	reservationCode := ""
	reservationStatus := ""
	checkInDate := ""
	checkOutDate := ""
	guests := 0
	if s.reservationRepo != nil {
		if res, err := s.reservationRepo.FindByID(ctx, c.ReservationID); err == nil && res != nil {
			reservationCode = res.Code
			reservationStatus = string(res.Status)
			if !res.CheckIn.IsZero() {
				checkInDate = res.CheckIn.Format("2006-01-02")
			}
			if !res.CheckOut.IsZero() {
				checkOutDate = res.CheckOut.Format("2006-01-02")
			}
			guests = res.Guests
			if hostID == uuid.Nil && res.HostID != uuid.Nil {
				hostID = res.HostID
				if host, err := s.userRepo.FindByID(ctx, res.HostID); err == nil && host != nil && host.FullName != "" {
					hostName = host.FullName
				}
			}
		}
	}

	latest, _ := s.messagingRepo.GetLatestMessage(ctx, c.ID)
	var latestResp *dtos.MessageResponse
	if latest != nil {
		r := s.toMessageResponse(ctx, *latest, nil)
		latestResp = &r
	}

	unreadCount, _ := s.messagingRepo.GetUnreadCount(ctx, c.ID, currentUserID)

	return &dtos.ConversationResponse{
		ID:                c.ID,
		ReservationID:     c.ReservationID,
		ReservationCode:   reservationCode,
		ReservationStatus: reservationStatus,
		CheckInDate:       checkInDate,
		CheckOutDate:      checkOutDate,
		Guests:            guests,
		ListingID:         c.ListingID,
		ListingTitle:      listingTitle,
		ListingCity:       listingCity,
		ListingCoverURL:   listingCoverURL,
		HostID:            hostID,
		HostName:          hostName,
		GuestID:           c.GuestID,
		GuestName:         guestName,
		LastMessage:       latestResp,
		UnreadCount:       unreadCount,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}, nil
}
