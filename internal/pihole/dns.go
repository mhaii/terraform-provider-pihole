package pihole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mhaii/go-pihole"
)

type DNSRecordsListResponse struct {
	Data [][]string
}

// ToDNSRecordList converts a DNSRecordsListResponse into a DNSRecordList object.
func (rr DNSRecordsListResponse) ToDNSRecordList() DNSRecordList {
	list := DNSRecordList{}

	for _, record := range rr.Data {
		list = append(list, DNSRecord{
			Domain: record[0],
			IP:     record[1],
		})
	}

	return list
}

type DNSRecordList = pihole.DNSRecordList
type DNSRecord = pihole.DNSRecord

// ListDNSRecords Returns the list of custom DNS records configured in pihole
func (c Client) ListDNSRecords(ctx context.Context) (DNSRecordList, error) {
	if c.tokenClient != nil {
		return c.tokenClient.LocalDNS.List(ctx)
	}

	req, err := c.RequestWithSession(ctx, "POST", "/admin/scripts/pi-hole/php/customdns.php", &url.Values{
		"action": []string{"get"},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var dnsRes DNSRecordsListResponse
	if err = json.NewDecoder(res.Body).Decode(&dnsRes); err != nil {
		return nil, err
	}

	return dnsRes.ToDNSRecordList(), nil
}

type CreateDNSRecordResponse struct {
	Success bool
	Message string
}

// CreateDNSRecord creates a pihole DNS record entry
func (c Client) CreateDNSRecord(ctx context.Context, record *DNSRecord) (*DNSRecord, error) {
	if c.tokenClient != nil {
		return c.tokenClient.LocalDNS.Create(ctx, record.Domain, record.IP)
	}

	req, err := c.RequestWithSession(ctx, "POST", "/admin/scripts/pi-hole/php/customdns.php", &url.Values{
		"action": []string{"add"},
		"ip":     []string{record.IP},
		"domain": []string{record.Domain},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var created CreateDNSRecordResponse
	if err = json.NewDecoder(res.Body).Decode(&created); err != nil {
		return nil, err
	}

	if !created.Success {
		return nil, fmt.Errorf(created.Message)
	}

	return record, nil
}

// GetDNSRecord searches the pihole local DNS records for the passed domain and returns first result if found
func (c Client) GetDNSRecord(ctx context.Context, domain string) (*DNSRecord, error) {
	if c.tokenClient != nil {
		record, err := c.tokenClient.LocalDNS.Get(ctx, domain)
		if err != nil {
			if errors.Is(err, pihole.ErrorLocalDNSNotFound) {
				return nil, NewNotFoundError(fmt.Sprintf("dns record with domain %q not found", domain))
			}

			return nil, err
		}

		return record, nil
	}

	list, err := c.GetDNSRecordList(ctx, domain)
	if err != nil {
		return nil, err
	}

	return list[0], nil
}

// GetDNSRecordList searches the pihole local DNS records for the passed domain and returns all result if found
func (c Client) GetDNSRecordList(ctx context.Context, domain string) ([]*DNSRecord, error) {
	if c.tokenClient != nil {
		records, err := c.tokenClient.LocalDNS.GetList(ctx, domain)
		if err != nil {
			if errors.Is(err, pihole.ErrorLocalDNSNotFound) {
				return nil, NewNotFoundError(fmt.Sprintf("dns record with domain %q not found", domain))
			}

			return nil, err
		}

		return records, nil
	}

	list, err := c.ListDNSRecords(ctx)
	if err != nil {
		return nil, err
	}

	var results []*DNSRecord
	for _, record := range list {
		if record.Domain == strings.ToLower(domain) {
			results = append(results, &record)
		}
	}

	if len(results) == 0 {
		return nil, NewNotFoundError(fmt.Sprintf("record %q not found", domain))
	}

	return results, nil
}

// DeleteDNSRecord deletes a pihole local DNS record by domain name
func (c Client) DeleteDNSRecord(ctx context.Context, domain string) error {
	if c.tokenClient != nil {
		return c.tokenClient.LocalDNS.Delete(ctx, domain)
	}

	records, err := c.GetDNSRecordList(ctx, domain)
	if err != nil {
		return err
	}

	for _, record := range records {
		if err = func() error {
			req, err := c.RequestWithSession(ctx, "POST", "/admin/scripts/pi-hole/php/customdns.php", &url.Values{
				"action": []string{"delete"},
				"ip":     []string{record.IP},
				"domain": []string{record.Domain},
			})
			if err != nil {
				return err
			}

			res, err := c.client.Do(req)
			if err != nil {
				return err
			}

			defer res.Body.Close()

			return nil
		}(); err != nil {
			return err
		}
	}

	return nil
}
