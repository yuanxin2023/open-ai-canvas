package app

import (
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/prompts"
)

type InspirationLibraryCommand = prompts.InspirationLibraryCommand
type InspirationLibraryView = prompts.InspirationLibraryView

func (s *Service) InspirationLibrary(user *model.User) (*InspirationLibraryView, error) {
	if user == nil || user.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureInspirationHall); err != nil {
		return nil, err
	}
	return s.promptDomain().InspirationLibrary(user.ID)
}

func (s *Service) ApplyInspirationLibrary(user *model.User, command InspirationLibraryCommand) (*InspirationLibraryView, error) {
	if user == nil || user.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureInspirationHall); err != nil {
		return nil, err
	}
	return s.promptDomain().ApplyInspirationLibrary(user.ID, command)
}
