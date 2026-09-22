package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type SubLinkProvider interface {
	SubLinksForSubId(host, subId string) ([]string, error)
	LinksForClient(host string, inbound *model.Inbound, email string) []string
	LinksForInbounds(host string, inbounds []*model.Inbound) []string
}

var registeredSubLinkProvider SubLinkProvider

func RegisterSubLinkProvider(p SubLinkProvider) {
	registeredSubLinkProvider = p
}

func (s *InboundService) GetSubLinks(host, subId string) ([]string, error) {
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	return registeredSubLinkProvider.SubLinksForSubId(host, subId)
}

func (s *InboundService) GetAllInboundLinks(host string, userId int) ([]string, error) {
	return s.GetAllInboundLinksForScope(host, InboundAccessScope{LegacyUserID: userId})
}

// GetAllInboundLinksForScope is GetAllInboundLinks restricted to an access scope.
func (s *InboundService) GetAllInboundLinksForScope(host string, scope InboundAccessScope) ([]string, error) {
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	inbounds, err := s.GetInboundsForScope(scope)
	if err != nil {
		return nil, err
	}
	return registeredSubLinkProvider.LinksForInbounds(host, inbounds), nil
}

func (s *InboundService) GetAllClientLinks(host string, email string) ([]string, error) {
	if email == "" {
		return nil, common.NewError("client email is required")
	}
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	rec, err := s.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	inboundIds, err := s.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return nil, err
	}
	var links []string
	for _, ibId := range inboundIds {
		inbound, getErr := s.GetInbound(ibId)
		if getErr != nil {
			return nil, getErr
		}
		links = append(links, registeredSubLinkProvider.LinksForClient(host, inbound, email)...)
	}
	return links, nil
}
