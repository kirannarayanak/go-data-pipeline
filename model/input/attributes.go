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
			} `json:"values"`
			PaymentMethods struct {
				Data []struct {
					Value string `json:"value"`
				} `json:"data"`
			} `json:"paymentMethods"`
		} `json:"data"`
	} `json:"stepsData"`
}

// FetchAds fetches ads from the Hasura GraphQL endpoint
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
			CodeNumber  json.Number     `json:"code_number"` // Handle GTIN as json.Number
			Attributes  json.RawMessage `json:"attributes"`
		} `json:"ads"`
	}

	err := client.Run(context.Background(), req, &response)
	if err != nil {
		return nil, err
	}

	var items []AdItem

	// Process the attributes of each ad
	for _, ad := range response.Ads {
		var attrs AdAttributes
		err := json.Unmarshal(ad.Attributes, &attrs)
		if err != nil {
			log.Printf("Error unmarshalling attributes for ad ID %s: %v", ad.ID, err)
			continue
		}

		// Check for payment method "Online Payment"
		hasOnlinePayment := false
		for _, step := range attrs.StepsData {
			if step.Name == "delivery_and_payment_methods" {
				for _, payment := range step.Data.PaymentMethods.Data {
					if payment.Value == "Online Payment" {
						hasOnlinePayment = true
						break
					}
				}
			}
		}

		// If "Online Payment" is found, process the ad
		if hasOnlinePayment {
			// Extract title, brand, price, and image src from attributes
			title, brand, price, imageSrc := "", "", "", ""
			for _, step := range attrs.StepsData {
				if step.Name == "search_product" {
					title = step.Data.InputSearchValue.Value
				} else if step.Name == "product_detail" {
					brand = step.Data.Values.Brand
					price = step.Data.Values.Price + " AED"
					if len(step.Data.Values.Images) > 0 {
						imageSrc = step.Data.Values.Images[0].Src
					}
				}
			}

			// Ensure that `imageSrc` is properly formatted without encoding issues
			if imageSrc != "" {
				// Generate the image URL
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
				ImageLink:    imageSrc, // Don't encode, leave the raw URL for XML generation
				Brand:        brand,
				Price:        price,
				Availability: "in stock",
				CodeNumber:   ad.CodeNumber, // Assign the GTIN here
			})
		}
	}

	return items, nil
}
