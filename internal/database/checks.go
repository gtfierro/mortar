package database

import (
	"errors"
	"fmt"

	"github.com/gtfierro/mortar2/internal/config"
	"github.com/knakk/rdf"
)

func checkConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("Configuration is nil")
	} else if len(cfg.Database.Host) == 0 {
		return errors.New("Database.Host is empty")
	} else if len(cfg.Database.Database) == 0 {
		return errors.New("Database.Database is empty")
	} else if len(cfg.Database.User) == 0 {
		return errors.New("Database.User is empty")
	} else if len(cfg.Database.Password) == 0 {
		return errors.New("Database.Password is empty")
	} else if len(cfg.Database.Port) == 0 {
		return errors.New("Database.Port is empty")
	}
	return nil
}

func checkStream(s *Stream) error {
	if s == nil {
		return errors.New("Stream is null")
	} else if len(s.SourceName) == 0 {
		return errors.New("SourceName is null")
	} else if len(s.Units) == 0 {
		return errors.New("Units is null")
	} else if len(s.Name) == 0 {
		return errors.New("Name is null")
	}

	// validate BrickURI
	if len(s.BrickURI) > 0 {
		if _, err := rdf.NewIRI(s.BrickURI); err != nil {
			return fmt.Errorf("BrickURI '%s' is invalid: %w", s.BrickURI, err)
		}
	}

	// validate BrickClass
	if len(s.BrickClass) > 0 {
		if _, err := rdf.NewIRI(s.BrickClass); err != nil {
			return fmt.Errorf("BrickClass '%s' is invalid: %w", s.BrickClass, err)
		}
	}

	return nil
}

// TODO: check if stream is registered
func checkDataset(d Dataset) error {
	if d == nil {
		return errors.New("Dataset is null")
	} else if len(d.GetSource()) == 0 {
		return errors.New("SourceName is null")
	} else if len(d.GetName()) == 0 {
		return errors.New("Name is null")
		//	} else if len(d.Readings) == 0 {
		//		return errors.New("Dataset is empty")
	}

	return nil
}
