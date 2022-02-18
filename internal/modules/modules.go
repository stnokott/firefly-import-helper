package modules

import (
	"firefly-iii-fix-ing/internal/structs"
	"log"
	"regexp"
	"strings"
)

type fixTransactionModule interface {
	process(s *structs.TransactionSplitUpdate) (*structs.TransactionSplitUpdate, error)
	shouldReturnOnSuccess() bool
	name() string
}

type ModuleHandler struct {
	moduleFuncs []fixTransactionModule
}

func NewModuleHandler() *ModuleHandler {
	log.Println("Loading modules...")
	moduleFuncs := []fixTransactionModule{
		&moduleLinebreaks{},
		&moduleIngDescriptionFormat{},
		&modulePaypalDescriptionFormat{},
	}
	for _, m := range moduleFuncs {
		log.Printf(">> [%s]", m.name())
	}
	return &ModuleHandler{moduleFuncs: moduleFuncs}
}

func mergeTransactionUpdates(src *structs.TransactionSplitUpdate, dst *structs.TransactionSplitUpdate, moduleName string) {
	updatedVals := map[string]string{}
	if src.CreditorId != "" {
		dst.CreditorId = src.CreditorId
		updatedVals["CreditorId"] = src.CreditorId
	}
	if src.MandateReference != "" {
		dst.MandateReference = src.MandateReference
		updatedVals["MandateReference"] = src.MandateReference
	}
	if src.Description != "" {
		dst.Description = src.Description
		updatedVals["Description"] = src.Description
	}
	for k, v := range updatedVals {
		if v != "" {
			log.Printf(">>>> [%s]: SET %s='%s'", moduleName, k, v)
		}
	}
}

func (mh *ModuleHandler) Process(s *structs.WhTransactionSplit) (*structs.TransactionSplitUpdate, error) {
	didUpdate := false
	finalUpdate := &structs.TransactionSplitUpdate{
		JournalId:   s.JournalId,
		Description: s.Description,
	}

	for _, module := range mh.moduleFuncs {
		update, err := module.process(finalUpdate)
		if err != nil {
			log.Printf(">>>> ERROR: [%s]: %s", module.name(), err)
		} else if update == nil {
			log.Printf(">>>> [%s]: not applicable", module.name())
		} else {
			mergeTransactionUpdates(update, finalUpdate, module.name())
			didUpdate = true
			if module.shouldReturnOnSuccess() {
				log.Printf(">>>> [%s]: returning updated transaction", module.name())
				return finalUpdate, nil
			}
		}
	}
	if didUpdate {
		return finalUpdate, nil
	} else {
		return nil, nil
	}
}

type moduleIngDescriptionFormat struct {
}

func (m *moduleIngDescriptionFormat) name() string {
	return "ING description format"
}

func (m *moduleIngDescriptionFormat) shouldReturnOnSuccess() bool {
	return true
}

var regexIngDescription = regexp.MustCompile(`^mandatereference:(.*),creditorid:(.*),remittanceinformation:(.*)$`)

func (m *moduleIngDescriptionFormat) process(s *structs.TransactionSplitUpdate) (*structs.TransactionSplitUpdate, error) {
	matches := regexIngDescription.FindStringSubmatch(s.Description)
	if matches == nil {
		return nil, nil
	} else {
		matches = matches[1:] // remove first entry containing the whole match
		description := matches[2]
		if description == "" {
			// revert if empty
			description = s.Description
		}
		return &structs.TransactionSplitUpdate{
			MandateReference: matches[0],
			CreditorId:       matches[1],
			Description:      description,
		}, nil
	}
}

type moduleLinebreaks struct {
}

func (m *moduleLinebreaks) name() string {
	return "Replace escaped linebreaks"
}

func (m *moduleLinebreaks) shouldReturnOnSuccess() bool {
	return false
}

func (m *moduleLinebreaks) process(s *structs.TransactionSplitUpdate) (*structs.TransactionSplitUpdate, error) {
	newDescription := strings.ReplaceAll(s.Description, "; ", "")
	if newDescription == s.Description {
		return nil, nil
	} else {
		return &structs.TransactionSplitUpdate{
			Description: newDescription,
		}, nil
	}
}

type modulePaypalDescriptionFormat struct {
}

func (m *modulePaypalDescriptionFormat) name() string {
	return "PayPal description format"
}

func (m *modulePaypalDescriptionFormat) shouldReturnOnSuccess() bool {
	return true
}

var regexPaypalDescription = regexp.MustCompile(`^PP\.\d{4}\.PP \. .+, Ihr (Einkauf bei .+)$`)

func (m *modulePaypalDescriptionFormat) process(s *structs.TransactionSplitUpdate) (*structs.TransactionSplitUpdate, error) {
	matches := regexPaypalDescription.FindStringSubmatch(s.Description)
	if matches == nil {
		return nil, nil
	} else {
		matches = matches[1:] // remove first entry containing the whole match
		return &structs.TransactionSplitUpdate{
			Description: "PayPal: " + matches[0],
		}, nil
	}
}
