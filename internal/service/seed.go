package service

import (
	"context"
	"fmt"

	"marketplace/internal/domain"
	"marketplace/internal/repository"
)

var catalog = []domain.Product{
	{SKU: "STEAM-TOPUP-500", Name: "Пополнение Steam 500 ₽", Type: "topup", Price: 500, Currency: "RUB", Image: "/assets/steam.svg"},
	{SKU: "STEAM-TOPUP-1000", Name: "Пополнение Steam 1000 ₽", Type: "topup", Price: 1000, Currency: "RUB", Image: "/assets/steam.svg"},
	{SKU: "STEAM-TOPUP-2500", Name: "Пополнение Steam 2500 ₽", Type: "topup", Price: 2500, Currency: "RUB", Image: "/assets/steam.svg"},
	{SKU: "KEY-CS2-PRIME", Name: "CS2 Prime Status ключ", Type: "key", Price: 1290, Currency: "RUB", Image: "/assets/cs2.svg"},
	{SKU: "KEY-GTA5", Name: "GTA V ключ активации", Type: "key", Price: 1990, Currency: "RUB", Image: "/assets/gta5.svg"},
	{SKU: "KEY-EFT", Name: "Escape from Tarkov ключ", Type: "key", Price: 3490, Currency: "RUB", Image: "/assets/eft.svg"},
	{SKU: "SUB-DISCORD-1M", Name: "Discord Nitro 1 месяц", Type: "subscription", Price: 399, Currency: "RUB", Image: "/assets/discord.svg"},
	{SKU: "SUB-YT-3M", Name: "YouTube Premium 3 месяца", Type: "subscription", Price: 1490, Currency: "RUB", Image: "/assets/youtube.svg"},
	{SKU: "SUB-SPOTIFY-1M", Name: "Spotify Premium 1 месяц", Type: "subscription", Price: 299, Currency: "RUB", Image: "/assets/spotify.svg"},
	{SKU: "GIFT-PSN-1000", Name: "PlayStation Store карта 1000 ₽", Type: "giftcard", Price: 1000, Currency: "RUB", Image: "/assets/psn.svg"},
	{SKU: "GIFT-XBOX-1500", Name: "Xbox Gift Card 1500 ₽", Type: "giftcard", Price: 1500, Currency: "RUB", Image: "/assets/xbox.svg"},
	{SKU: "GIFT-ROBLOX-800", Name: "Roblox 800 Robux", Type: "giftcard", Price: 890, Currency: "RUB", Image: "/assets/roblox.svg"},
}

var keys = []string{
	"LFXC-TNCS-BPCD", "P3EI-W8UO-9B4K", "FEL3-GUXN-TCCH", "YPLV-QK2Z-IUS5", "0K9E-P1FR-BY1U",
	"5LZV-UQ48-RXCZ", "X93K-NYAQ-GEC1", "EIO5-CQT5-35KO", "M58F-GIIR-VJAP", "NU8Y-SWYB-6252",
	"OODW-CCHF-MBAF", "DNA5-WFJM-NE49", "QRDD-MJ3F-A8TF", "TAT9-5ZJN-G1T2", "LI39-4330-ISMB",
	"BKJY-8Q79-8NHI", "HHW6-4RX2-DX62", "1RG2-L28O-O80G", "EF63-F39X-MTEA", "8XS7-P53H-JKIV",
	"JPE6-MQV6-P7ST", "SAPG-A2GR-0ULS", "T2DU-IJ1S-U16P", "WSSY-QTR7-Z57J", "U74E-EPCI-CY26",
	"FZXF-58H8-OR93", "FPSM-HLZA-TPAL", "WSC9-28DJ-B2JE", "P63J-F7UZ-DCYP", "C7W2-D4C5-QMT7",
	"JESI-DFBH-LK1K", "SGMA-JA0T-GR7D", "3PR4-OSY9-M3ZW", "OMBE-C0JF-D45Y", "KIKQ-FQJ8-9TI8",
	"LMAN-RSHS-AJDO", "BAKI-VT1X-Z5OL", "9F0X-B46W-03FS", "S423-V6YY-IBEM", "D4UW-WYRA-20ST",
	"XC0J-CJ0H-09RN", "RY1W-XCFJ-0KUA", "CJYY-YKSQ-QE6H", "97AQ-38QJ-H8HU", "FS8E-3S5Z-I6RA",
	"ARQK-FML4-A14E", "7Z6K-NO9V-MPJB", "D4K7-IJSG-N853", "W67T-ZB0Q-1XKB", "7EQM-K09J-XKUO",
}

var keyQuota = []int{5, 5, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}

type Seeder struct {
	products  *repository.ProductRepo
	inventory *repository.InventoryRepo
}

func NewSeeder(products *repository.ProductRepo, inventory *repository.InventoryRepo) *Seeder {
	return &Seeder{products: products, inventory: inventory}
}

func (s *Seeder) Run(ctx context.Context, extra int) error {
	for _, p := range catalog {
		if err := s.products.Upsert(ctx, p); err != nil {
			return err
		}
	}
	idx := 0
	for i, p := range catalog {
		n := keyQuota[i]
		for j := 0; j < n && idx < len(keys); j++ {
			if err := s.inventory.InsertKey(ctx, p.SKU, keys[idx]); err != nil {
				return err
			}
			idx++
		}
		if err := s.products.SyncStock(ctx, p.SKU); err != nil {
			return err
		}
	}
	for i := 0; i < extra; i++ {
		sku := fmt.Sprintf("GEN-%04d", i)
		p := domain.Product{
			SKU:      sku,
			Name:     fmt.Sprintf("Digital item %04d", i),
			Type:     []string{"topup", "key", "subscription", "giftcard"}[i%4],
			Price:    99 + (i%50)*10,
			Currency: "RUB",
			Image:    "/assets/generic.svg",
			Stock:    1 + i%7,
		}
		if err := s.products.Upsert(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
