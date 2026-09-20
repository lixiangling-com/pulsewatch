package monitor

import (
	"context"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, input CreateInput) (Monitor, error) {
	normalized, err := validateCreate(input)
	if err != nil {
		return Monitor{}, err
	}
	now := s.now().UTC()
	return s.store.Create(ctx, userID, Monitor{
		ID:              uuid.New(),
		UserID:          userID,
		Name:            normalized.Name,
		URL:             normalized.URL,
		IntervalMinutes: normalized.IntervalMinutes,
		ExpectedStatus:  normalized.ExpectedStatus,
		Status:          StatusPending,
		ConfigVersion:   1,
		NextCheckAt:     now,
	})
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, page, pageSize int) (Page, error) {
	if page < 1 {
		return Page{}, validation(map[string][]string{"page": {"页码必须是正整数"}})
	}
	if pageSize < 1 || pageSize > 100 {
		return Page{}, validation(map[string][]string{"page_size": {"每页数量应在 1 到 100 之间"}})
	}
	const maxInt32 = int(^uint32(0) >> 1)
	if page-1 > maxInt32/pageSize {
		return Page{}, validation(map[string][]string{"page": {"页码超出范围"}})
	}
	items, total, err := s.store.List(ctx, userID, page, pageSize)
	return Page{Items: items, Number: page, Size: pageSize, Total: total}, err
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Monitor, error) {
	return s.store.Get(ctx, userID, id)
}

func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, input UpdateInput) (Monitor, error) {
	validated, err := validateUpdate(input)
	if err != nil {
		return Monitor{}, err
	}
	return s.store.Mutate(ctx, userID, id, func(current Monitor) (Monitor, error) {
		configurationChanged := false
		if validated.Name != nil {
			current.Name = *validated.Name
		}
		if validated.URL != nil && current.URL != *validated.URL {
			current.URL = *validated.URL
			configurationChanged = true
		}
		if validated.IntervalMinutes != nil && current.IntervalMinutes != *validated.IntervalMinutes {
			current.IntervalMinutes = *validated.IntervalMinutes
			configurationChanged = true
		}
		if validated.ExpectedStatus != nil && current.ExpectedStatus != *validated.ExpectedStatus {
			current.ExpectedStatus = *validated.ExpectedStatus
			configurationChanged = true
		}
		if configurationChanged {
			current.ConfigVersion++
			if current.Status != StatusPaused {
				current.Status = StatusPending
				current.NextCheckAt = s.now().UTC()
			}
		}
		return current, nil
	})
}

func (s *Service) Pause(ctx context.Context, userID, id uuid.UUID) (Monitor, error) {
	return s.store.Mutate(ctx, userID, id, func(current Monitor) (Monitor, error) {
		if current.Status != StatusPaused {
			current.Status = StatusPaused
			current.ConfigVersion++
		}
		return current, nil
	})
}

func (s *Service) Resume(ctx context.Context, userID, id uuid.UUID) (Monitor, error) {
	return s.store.Mutate(ctx, userID, id, func(current Monitor) (Monitor, error) {
		if current.Status == StatusPaused {
			current.Status = StatusPending
			current.ConfigVersion++
			current.NextCheckAt = s.now().UTC()
		}
		return current, nil
	})
}

func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.store.Delete(ctx, userID, id)
}

func validateCreate(input CreateInput) (CreateInput, error) {
	fields := map[string][]string{}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	validateName(input.Name, fields)
	validateURL(input.URL, fields)
	validateInterval(input.IntervalMinutes, fields)
	validateExpectedStatus(input.ExpectedStatus, fields)
	if len(fields) > 0 {
		return CreateInput{}, validation(fields)
	}
	return input, nil
}

func validateUpdate(input UpdateInput) (UpdateInput, error) {
	if input.Name == nil && input.URL == nil && input.IntervalMinutes == nil && input.ExpectedStatus == nil {
		return UpdateInput{}, validation(map[string][]string{"body": {"至少提供一个可修改字段"}})
	}
	fields := map[string][]string{}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		input.Name = &value
		validateName(value, fields)
	}
	if input.URL != nil {
		value := strings.TrimSpace(*input.URL)
		input.URL = &value
		validateURL(value, fields)
	}
	if input.IntervalMinutes != nil {
		validateInterval(*input.IntervalMinutes, fields)
	}
	if input.ExpectedStatus != nil {
		validateExpectedStatus(*input.ExpectedStatus, fields)
	}
	if len(fields) > 0 {
		return UpdateInput{}, validation(fields)
	}
	return input, nil
}

func validateName(value string, fields map[string][]string) {
	if size := utf8.RuneCountInString(value); size < 1 || size > 80 || strings.ContainsRune(value, '\x00') {
		fields["name"] = []string{"名称长度应为 1 到 80 个字符"}
	}
}

func validateInterval(value int, fields map[string][]string) {
	if value != 1 && value != 5 && value != 10 {
		fields["interval_minutes"] = []string{"检查间隔只能是 1、5 或 10 分钟"}
	}
}

func validateExpectedStatus(value int, fields map[string][]string) {
	if value < 100 || value > 599 {
		fields["expected_status"] = []string{"预期状态码应在 100 到 599 之间"}
	}
}

func validateURL(value string, fields map[string][]string) {
	if size := utf8.RuneCountInString(value); size < 1 || size > 2048 {
		fields["url"] = []string{"URL 长度应为 1 到 2048 个字符"}
		return
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		fields["url"] = []string{"请输入有效的公开 HTTP 或 HTTPS URL"}
		return
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" || parsed.User != nil || parsed.Fragment != "" {
		fields["url"] = []string{"请输入有效的公开 HTTP 或 HTTPS URL"}
		return
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" || host == "localhost" || host == "local" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.Contains(host, "%") {
		fields["url"] = []string{"URL 必须指向公开地址"}
		return
	}
	if parsed.Port() != "" {
		if _, err := netip.ParseAddrPort("127.0.0.1:" + parsed.Port()); err != nil {
			fields["url"] = []string{"URL 端口无效"}
			return
		}
	}
	if address, err := netip.ParseAddr(host); err == nil && !publicAddress(address.Unmap()) {
		fields["url"] = []string{"URL 必须指向公开地址"}
	}
}

func publicAddress(address netip.Addr) bool {
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsUnspecified() || address.IsMulticast() {
		return false
	}
	for _, prefix := range blockedPublicLookingPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

var blockedPublicLookingPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func validation(fields map[string][]string) error {
	return &ValidationError{Fields: fields}
}
