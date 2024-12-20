package input

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/machinebox/graphql"
)

// AdItem represents the structure for storing ad information
type AdItem struct {
	ID           string
	Title        string
	Description  string
	Link         string
	ImageLink    string
	Brand        string
	Price        string
	Availability string
	CodeNumber   json.Number // Handle GTIN as json.Number
}

// AdAttributes represents the structure of attributes for each ad
type AdAttributes struct {
	StepsData []struct {
		Name string `json:"name"`
		Data struct {
			InputSearchValue struct {
				Value string `json:"value"`
			} `json:"inputSearchValue"`
			Values struct {
				Brand  string `json:"brand"`
				Price  string `json:"price"`
				Images []struct {
					Src string `json:"src"`
				} `json:"images"`
				AdType string `json:"ad_type"`
			} `json:"values"`
			PaymentMethods struct {
				Data []struct {
					Value string `json:"value"`
				} `json:"data"`
			} `json:"paymentMethods"`
		} `json:"data"`
	} `json:"stepsData"`
}

func FetchAds(endpoint, adminSecret string) ([]AdItem, error) {
	client := graphql.NewClient(endpoint)

	// GraphQL query with status, category, and payment method filter
	req := graphql.NewRequest(`
	query ($last24Hours: timestamptz!) {
		ads(where: {
			status: {_eq: "Published"},
			category_id: {_eq: "9ca82557-9085-40da-82db-c9a3c3d3f3a6"},
			updated_at: { _gte: $last24Hours }
		}) {
			id
			draft_id
			description
			attributes
			code_number
		}
	}
`)

	// Calculate the timestamp for the last 24 hours
	last24Hours := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	req.Var("last24Hours", last24Hours)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hasura-Admin-Secret", adminSecret)

	var response struct {
		Ads []struct {
			ID          string          `json:"id"`
			DraftID     string          `json:"draft_id"`
			Description string          `json:"description"`
			CodeNumber  json.Number     `json:"code_number"`
			Attributes  json.RawMessage `json:"attributes"`
		} `json:"ads"`
	}

	err := client.Run(context.Background(), req, &response)
	if err != nil {
		return nil, err
	}

	var items []AdItem
	auctionCount := 0
	otherCount := 0

	// Process the attributes of each ad
	for _, ad := range response.Ads {
		var attrs AdAttributes
		err := json.Unmarshal(ad.Attributes, &attrs)
		if err != nil {
			log.Printf("Error unmarshalling attributes for ad ID %s: %v", ad.ID, err)
			continue
		}

		// Check for ad_type and count "auction" vs other types
		adType := ""
		price := ""
		hasOnlinePayment := false
		for _, step := range attrs.StepsData {
			if step.Name == "delivery_and_payment_methods" {
				for _, payment := range step.Data.PaymentMethods.Data {
					if payment.Value == "Online Payment" {
						hasOnlinePayment = true
					}
				}
			} else if step.Name == "product_detail" {
				adType = step.Data.Values.AdType
				price = step.Data.Values.Price
			}
		}

		// Skip ads with an empty price
		if price == "" {
			log.Printf("Skipping ad with ID %s due to empty price field.", ad.ID)
			continue
		}

		// Count ad types
		if adType == "auction" {
			auctionCount++
		} else {
			otherCount++
		}

		// If "Online Payment" is found and no "Auctions" in attributes, process the ad
		if hasOnlinePayment {
			// Extract title, brand, and image src from attributes
			title, brand, imageSrc := "", "", ""
			for _, step := range attrs.StepsData {
				if step.Name == "search_product" {
					title = step.Data.InputSearchValue.Value
				} else if step.Name == "product_detail" {
					brand = step.Data.Values.Brand
					if len(step.Data.Values.Images) > 0 {
						imageSrc = step.Data.Values.Images[0].Src
					}
				}
			}

			// Ensure that `imageSrc` is properly formatted without encoding issues
			if imageSrc != "" {
				imageSrc = fmt.Sprintf(
					"https://ayshei.com/_next/image?url=https://storage.ayshei.com/prod/public/drafts/%s/web/%s&amp;w=3840&amp;q=75",
					ad.DraftID, imageSrc)
			}

			// Build the AdItem
			items = append(items, AdItem{
				ID:           ad.ID,
				Title:        title,
				Description:  ad.Description,
				Link:         fmt.Sprintf("https://ayshei.com/product/%s", ad.ID),
				ImageLink:    imageSrc,
				Brand:        brand,
				Price:        price + " AED",
				Availability: "in stock",
				CodeNumber:   ad.CodeNumber,
			})
		}
	}

	// Log counts of "auction" and other ad types
	log.Printf("Total ads with ad_type 'auction': %d", auctionCount)
	log.Printf("Total ads with other ad types: %d", otherCount)

	return items, nil
}
