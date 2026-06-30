package models

import (
	"time"
)

// ---- Enumerations ----------------------------------------------------------

// CampaignStatus tracks the lifecycle of an ad campaign.
type CampaignStatus string

const (
	CampaignStatusDraft    CampaignStatus = "draft"
	CampaignStatusReview   CampaignStatus = "review"
	CampaignStatusActive   CampaignStatus = "active"
	CampaignStatusPaused   CampaignStatus = "paused"
	CampaignStatusEnded    CampaignStatus = "ended"
	CampaignStatusRejected CampaignStatus = "rejected"
)

// CampaignObjective is the advertiser's primary goal.
type CampaignObjective string

const (
	ObjectiveAwareness   CampaignObjective = "awareness"
	ObjectiveTraffic     CampaignObjective = "traffic"
	ObjectiveEngagement  CampaignObjective = "engagement"
	ObjectiveLeads       CampaignObjective = "leads"
	ObjectiveAppInstall  CampaignObjective = "app_install"
	ObjectiveConversions CampaignObjective = "conversions"
	ObjectiveVideoViews  CampaignObjective = "video_views"
)

// BillingEvent defines what counts as a billable action.
type BillingEvent string

const (
	BillingCPM  BillingEvent = "cpm"  // cost per 1000 impressions
	BillingCPC  BillingEvent = "cpc"  // cost per click
	BillingCPV  BillingEvent = "cpv"  // cost per video view
	BillingCPCA BillingEvent = "cpca" // cost per completed action
)

// AdFormat describes the creative format.
type AdFormat string

const (
	FormatTopView   AdFormat = "top_view"    // first video seen on open
	FormatInFeed    AdFormat = "in_feed"     // native feed placement
	FormatBrandTakeover AdFormat = "brand_takeover"
	FormatSparkAds  AdFormat = "spark_ads"   // boosted organic posts
	FormatBrandedHashtag AdFormat = "branded_hashtag_challenge"
	FormatBrandedEffect  AdFormat = "branded_effect"
)

// AdStatus is the lifecycle state of a single creative ad.
type AdStatus string

const (
	AdStatusDraft    AdStatus = "draft"
	AdStatusReview   AdStatus = "review"
	AdStatusActive   AdStatus = "active"
	AdStatusPaused   AdStatus = "paused"
	AdStatusRejected AdStatus = "rejected"
)

// Gender targeting options.
type Gender string

const (
	GenderAll    Gender = "all"
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// ---- Core entities ---------------------------------------------------------

// Campaign is the top-level container for an advertising effort.
type Campaign struct {
	ID           string            `json:"id" db:"id"`
	AdvertiserID string            `json:"advertiser_id" db:"advertiser_id"`
	Name         string            `json:"name" db:"name"`
	Objective    CampaignObjective `json:"objective" db:"objective"`
	Status       CampaignStatus    `json:"status" db:"status"`

	// Scheduling.
	StartTime time.Time  `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`

	// Budget reference.
	BudgetID string `json:"budget_id" db:"budget_id"`

	// Aggregated stats (denormalised for dashboard performance).
	TotalSpendMicroUSD int64   `json:"total_spend_micro_usd" db:"total_spend_micro_usd"`
	TotalImpressions   int64   `json:"total_impressions" db:"total_impressions"`
	TotalClicks        int64   `json:"total_clicks" db:"total_clicks"`
	TotalConversions   int64   `json:"total_conversions" db:"total_conversions"`
	AverageCTR         float64 `json:"average_ctr" db:"average_ctr"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// AdSet groups ads with shared targeting and budget pacing settings.
type AdSet struct {
	ID         string         `json:"id" db:"id"`
	CampaignID string         `json:"campaign_id" db:"campaign_id"`
	Name       string         `json:"name" db:"name"`
	Status     CampaignStatus `json:"status" db:"status"`

	// Targeting spec (points to an AudienceSegment or inline spec).
	AudienceSegmentID string          `json:"audience_segment_id,omitempty" db:"audience_segment_id"`
	Targeting         TargetingSpec   `json:"targeting" db:"targeting"`

	// Delivery window within the parent campaign's dates.
	StartTime *time.Time `json:"start_time,omitempty" db:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`

	// Placement.
	Placements  []string `json:"placements" db:"placements"`
	BillingEvent BillingEvent `json:"billing_event" db:"billing_event"`

	// Bid.
	BidAmountMicroUSD int64 `json:"bid_amount_micro_usd" db:"bid_amount_micro_usd"`

	// Daily budget (override from campaign budget).
	DailyBudgetMicroUSD int64 `json:"daily_budget_micro_usd,omitempty" db:"daily_budget_micro_usd"`

	// Frequency cap: max impressions per user per day.
	FrequencyCapPerDay int `json:"frequency_cap_per_day" db:"frequency_cap_per_day"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TargetingSpec holds inline demographic and interest targeting parameters.
type TargetingSpec struct {
	// Demographics.
	MinAge   int      `json:"min_age,omitempty"`
	MaxAge   int      `json:"max_age,omitempty"`
	Genders  []Gender `json:"genders,omitempty"`

	// Geographic targeting (ISO 3166-1 alpha-2 country codes or city IDs).
	Countries []string `json:"countries,omitempty"`
	Cities    []string `json:"cities,omitempty"`
	Regions   []string `json:"regions,omitempty"`

	// Interest categories (taxonomy IDs).
	IncludeInterests []string `json:"include_interests,omitempty"`
	ExcludeInterests []string `json:"exclude_interests,omitempty"`

	// Device.
	DeviceTypes     []string `json:"device_types,omitempty"`    // ios, android, web
	OSVersions      []string `json:"os_versions,omitempty"`
	ConnectionTypes []string `json:"connection_types,omitempty"` // wifi, cellular

	// Language.
	Languages []string `json:"languages,omitempty"`

	// Lookalike: source audience ID.
	LookalikeAudienceID string `json:"lookalike_audience_id,omitempty"`

	// Retargeting: re-engage users who engaged with a video or profile.
	RetargetingAudienceID string `json:"retargeting_audience_id,omitempty"`

	// Custom audience (first-party data upload).
	CustomAudienceIDs []string `json:"custom_audience_ids,omitempty"`
}

// Ad is a single creative unit within an ad set.
type Ad struct {
	ID        string   `json:"id" db:"id"`
	AdSetID   string   `json:"ad_set_id" db:"ad_set_id"`
	Name      string   `json:"name" db:"name"`
	Status    AdStatus `json:"status" db:"status"`
	Format    AdFormat `json:"format" db:"format"`

	// Creative assets.
	VideoURL     string `json:"video_url,omitempty" db:"video_url"`
	ImageURL     string `json:"image_url,omitempty" db:"image_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	Headline     string `json:"headline,omitempty" db:"headline"`
	Description  string `json:"description,omitempty" db:"description"`
	CtaText      string `json:"cta_text,omitempty" db:"cta_text"`
	DestinationURL string `json:"destination_url,omitempty" db:"destination_url"`

	// Tracking pixels.
	ImpressionTrackingURL string `json:"impression_tracking_url,omitempty" db:"impression_tracking_url"`
	ClickTrackingURL      string `json:"click_tracking_url,omitempty" db:"click_tracking_url"`

	// Review.
	ReviewNotes string `json:"review_notes,omitempty" db:"review_notes"`

	// Per-ad performance stats.
	Impressions  int64   `json:"impressions" db:"impressions"`
	Clicks       int64   `json:"clicks" db:"clicks"`
	CTR          float64 `json:"ctr" db:"ctr"`
	SpendMicroUSD int64  `json:"spend_micro_usd" db:"spend_micro_usd"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// AudienceSegment is a reusable, named audience definition.
type AudienceSegment struct {
	ID           string `json:"id" db:"id"`
	AdvertiserID string `json:"advertiser_id" db:"advertiser_id"`
	Name         string `json:"name" db:"name"`
	Description  string `json:"description,omitempty" db:"description"`

	// Type: custom | lookalike | interest | retargeting | demographic
	SegmentType string        `json:"segment_type" db:"segment_type"`
	Targeting   TargetingSpec `json:"targeting" db:"targeting"`

	// Estimated size (users matching this segment).
	EstimatedSize int64 `json:"estimated_size" db:"estimated_size"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Budget is a financial allocation attached to a campaign or ad set.
type Budget struct {
	ID           string `json:"id" db:"id"`
	CampaignID   string `json:"campaign_id" db:"campaign_id"`
	AdvertiserID string `json:"advertiser_id" db:"advertiser_id"`

	// Total lifetime budget. 0 means unlimited.
	LifetimeBudgetMicroUSD int64 `json:"lifetime_budget_micro_usd" db:"lifetime_budget_micro_usd"`
	DailyBudgetMicroUSD    int64 `json:"daily_budget_micro_usd" db:"daily_budget_micro_usd"`

	// Spend tracking.
	TotalSpendMicroUSD int64 `json:"total_spend_micro_usd" db:"total_spend_micro_usd"`
	TodaySpendMicroUSD int64 `json:"today_spend_micro_usd" db:"today_spend_micro_usd"`

	// Pacing: how much budget should ideally be spent per hour.
	HourlyBudgetMicroUSD int64 `json:"hourly_budget_micro_usd" db:"hourly_budget_micro_usd"`

	// Billing start/end for the current cycle.
	CycleStartDate time.Time `json:"cycle_start_date" db:"cycle_start_date"`
	CycleEndDate   time.Time `json:"cycle_end_date" db:"cycle_end_date"`

	// Whether the campaign is currently budget-limited (paused by pacer).
	BudgetExhausted bool `json:"budget_exhausted" db:"budget_exhausted"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Impression records a single ad display event.
type Impression struct {
	ID           string    `json:"id" db:"id"`
	AdID         string    `json:"ad_id" db:"ad_id"`
	AdSetID      string    `json:"ad_set_id" db:"ad_set_id"`
	CampaignID   string    `json:"campaign_id" db:"campaign_id"`
	AdvertiserID string    `json:"advertiser_id" db:"advertiser_id"`
	UserID       string    `json:"user_id" db:"user_id"`

	// Auction outcome.
	BidMicroUSD    int64   `json:"bid_micro_usd" db:"bid_micro_usd"`
	ChargeMicroUSD int64   `json:"charge_micro_usd" db:"charge_micro_usd"`
	PredictedCTR   float64 `json:"predicted_ctr" db:"predicted_ctr"`
	ECPMMicroUSD   int64   `json:"ecpm_micro_usd" db:"ecpm_micro_usd"`

	// Context.
	Placement   string `json:"placement" db:"placement"`
	DeviceType  string `json:"device_type" db:"device_type"`
	Country     string `json:"country" db:"country"`
	Platform    string `json:"platform" db:"platform"`

	ImpressionedAt time.Time `json:"impressioned_at" db:"impressioned_at"`
}

// Click records a user click on an ad.
type Click struct {
	ID           string    `json:"id" db:"id"`
	ImpressionID string    `json:"impression_id" db:"impression_id"`
	AdID         string    `json:"ad_id" db:"ad_id"`
	AdSetID      string    `json:"ad_set_id" db:"ad_set_id"`
	CampaignID   string    `json:"campaign_id" db:"campaign_id"`
	UserID       string    `json:"user_id" db:"user_id"`

	DestinationURL string `json:"destination_url" db:"destination_url"`
	DeviceType     string `json:"device_type" db:"device_type"`
	Country        string `json:"country" db:"country"`

	ClickedAt time.Time `json:"clicked_at" db:"clicked_at"`
}

// CampaignStats holds aggregated performance metrics for dashboard display.
type CampaignStats struct {
	CampaignID   string `json:"campaign_id"`
	CampaignName string `json:"campaign_name"`

	// Volume.
	Impressions  int64 `json:"impressions"`
	Clicks       int64 `json:"clicks"`
	Conversions  int64 `json:"conversions"`
	VideoViews   int64 `json:"video_views"`
	Reach        int64 `json:"reach"` // unique users reached

	// Rates.
	CTR          float64 `json:"ctr"`
	CVR          float64 `json:"cvr"`  // conversion rate
	ViewRate     float64 `json:"view_rate"`

	// Costs.
	TotalSpendMicroUSD int64   `json:"total_spend_micro_usd"`
	CPMMicroUSD        int64   `json:"cpm_micro_usd"`
	CPCMicroUSD        int64   `json:"cpc_micro_usd"`
	CPAMicroUSD        int64   `json:"cpa_micro_usd"`

	// Budget utilisation (0.0 – 1.0).
	BudgetUtilisation float64 `json:"budget_utilisation"`

	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// AdRequest is the payload from the feed service requesting an ad to show.
type AdRequest struct {
	UserID      string   `json:"user_id"`
	DeviceType  string   `json:"device_type"`
	Country     string   `json:"country"`
	Language    string   `json:"language"`
	Platform    string   `json:"platform"` // ios, android, web
	Placement   string   `json:"placement"` // in_feed, top_view, etc.
	OSVersion   string   `json:"os_version,omitempty"`
	ConnectionType string `json:"connection_type,omitempty"`
	// The video content currently playing (for contextual targeting).
	CurrentVideoID string `json:"current_video_id,omitempty"`
	// Max number of ads to return (default 1).
	Count int `json:"count,omitempty"`
}

// AdResponse is the winning ad returned to the feed service.
type AdResponse struct {
	Ad             *Ad     `json:"ad"`
	ImpressionID   string  `json:"impression_id"`
	TrackingPixel  string  `json:"tracking_pixel"`
	ChargeEstimate int64   `json:"charge_estimate_micro_usd"`
	ECPM           int64   `json:"ecpm_micro_usd"`
}
