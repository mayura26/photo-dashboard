package payouts

import "testing"

func TestParseExtractFencedMoney(t *testing.T) {
	raw := "```json\n{\"amount\":\"$1,520.40\",\"currency\":\"usd\",\"date\":\"October 12, 2026\",\"account\":\"\\\"FTMO-10482\\\"\",\"firmId\":\"nope\"}\n```"
	got, ok := ParseExtract(raw)
	if !ok {
		t.Fatal("expected parse")
	}
	if got.Amount == nil || *got.Amount != 1520.4 {
		t.Fatalf("amount: %v", got.Amount)
	}
	if got.Currency != "USD" {
		t.Fatalf("currency: %q", got.Currency)
	}
	if got.Date != "2026-10-12" {
		t.Fatalf("date: %q", got.Date)
	}
	if got.Account != "FTMO-10482" {
		t.Fatalf("account: %q", got.Account)
	}
	if got.FirmID != "nope" {
		t.Fatalf("firm: %q", got.FirmID)
	}
}

func TestParseExtractNullAmount(t *testing.T) {
	got, ok := ParseExtract(`{"amount":null,"currency":"","date":"","account":"","firmId":""}`)
	if !ok {
		t.Fatal("expected parse")
	}
	if got.Amount != nil {
		t.Fatalf("amount: %v", *got.Amount)
	}
	if got.Currency != "" || got.Date != "" {
		t.Fatalf("%+v", got)
	}
}

func TestParseExtractRejectsGarbage(t *testing.T) {
	if _, ok := ParseExtract("not json"); ok {
		t.Fatal("expected failure")
	}
}
