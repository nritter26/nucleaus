package web

import "net/http"

// librarySection is one thematic shelf of a language library.
type librarySection struct {
	Label string
	Blurb string
	Tag   string
	Items []link
}

// libraryShelf is one language inside the library archive.
type libraryShelf struct {
	Slug     string
	Name     string
	Logo     string
	Desc     string
	Sections []librarySection
}

func repo(name, desc, url string) link {
	l := link{Name: name, Desc: desc, URL: url, NewTab: true}
	l.Initials = initials(name)
	return l
}

// libraryShelves lists the languages exposed by the library, each with its
// curated repository shelves.
var libraryShelves = []libraryShelf{
	{
		Slug: "golang",
		Name: "Golang",
		Logo: "golang",
		Desc: "Go repositories, scaffolds, and fintech-grade money code.",
		Sections: []librarySection{
			{
				Label: "Go engineering",
				Blurb: "Starters and canonical layouts for production services.",
				Tag:   "01 / BUILD",
				Items: []link{
					repo("go-backend-clean-architecture", "Clean-architecture Go REST service reference.", "https://github.com/amitshekhariitbhu/go-backend-clean-architecture"),
					repo("project-layout", "Canonical Go project directory layout.", "https://github.com/golang-standards/project-layout"),
					repo("godev-kit", "Batteries-included Go backend starter kit.", "https://github.com/ducnpdev/godev-kit"),
				},
			},
			{
				Label: "Go libraries",
				Blurb: "The standard building blocks for Go backends.",
				Tag:   "02 / LIBRARY",
				Items: []link{
					repo("Cobra", "CLI framework for commands, flags, and docs.", "https://github.com/spf13/cobra"),
					repo("Zerolog", "Structured, zero-allocation logging.", "https://github.com/rs/zerolog"),
					repo("Testify", "Assertions, mocks, and suites for testing.", "https://github.com/stretchr/testify"),
					repo("Viper", "Configuration, environment, and remote backends.", "https://github.com/spf13/viper"),
					repo("Awesome Go", "Curated list of Go libraries and tools.", "https://github.com/avelino/awesome-go"),
					repo("Go Kit", "Microservice toolkit for transport, logging, and metrics.", "https://github.com/go-kit/kit"),
				},
			},
			{
				Label: "Fintech",
				Blurb: "Money-safe math, ledgers, protocols, and market plumbing.",
				Tag:   "03 / FINTECH",
				Items: []link{
					repo("go-micro", "Microservice framework with RPC, discovery, and pub/sub.", "https://github.com/go-micro/go-micro"),
					repo("awesome-fintech", "Curated fintech engineering resources.", "https://github.com/moov-io/awesome-fintech"),
					repo("finance-go", "Technical analysis and market data classes.", "https://github.com/piquette/finance-go"),
					repo("golang/crypto", "Supplemental cryptography packages.", "https://github.com/golang/crypto"),
					repo("go-ethereum", "Official Go implementation of the Ethereum protocol.", "https://github.com/ethereum/go-ethereum"),
					repo("cosmos-sdk", "Framework for building Proof-of-Stake chains.", "https://github.com/cosmos/cosmos-sdk"),
					repo("plaid-go", "Plaid API clients for bank account linking.", "https://github.com/plaid/plaid-go"),
					repo("gocryptotrader", "Crypto exchange trading bot framework.", "https://github.com/thrasher-corp/gocryptotrader"),
					repo("gonum", "Numerical computing for scientific finance.", "https://github.com/gonum/gonum"),
					repo("watchman", "Sanctions screening for financial entities.", "https://github.com/moov-io/watchman"),
					repo("stripe-go", "Official Stripe API bindings.", "https://github.com/stripe/stripe-go"),
					repo("paygate", "Open source ACH payment gateway.", "https://github.com/moov-io/paygate"),
					repo("ach", "ACH file reader, writer, and validator.", "https://github.com/moov-io/ach"),
					repo("iso8583", "ISO 8583 financial transaction message library.", "https://github.com/moov-io/iso8583"),
					repo("go-money", "Currency-safe money arithmetic.", "https://github.com/rhymond/go-money"),
					repo("decimal", "Arbitrary-precision fixed-point decimals.", "https://github.com/shopspring/decimal"),
					repo("go-finance", "Financial market data and analytics helpers.", "https://github.com/alpeb/go-finance"),
				},
			},
		},
	},
	{
		Slug: "typescript",
		Name: "TypeScript",
		Logo: "ts",
		Desc: "End-to-end typed APIs and edge-first routing.",
		Sections: []librarySection{
			{
				Label: "TypeScript libraries",
				Blurb: "Typed API layers and ultrafast request handling.",
				Tag:   "01 / BUILD",
				Items: []link{
					repo("tRPC", "End-to-end typesafe APIs without schemas.", "https://github.com/trpc/trpc"),
					repo("Hono", "Ultrafast web framework that runs anywhere.", "https://github.com/honojs/hono"),
				},
			},
		},
	},
	{
		Slug: "bash",
		Name: "Bash",
		Logo: "bash",
		Desc: "Script tooling, schema-driven CLIs, and shell test harnesses.",
		Sections: []librarySection{
			{
				Label: "Bash tooling",
				Blurb: "Libraries and utilities for serious shell work.",
				Tag:   "01 / BUILD",
				Items: []link{
					repo("lobash", "Modern bash library with 500+ helpers.", "https://github.com/adoyle-h/lobash"),
					repo("Bashly", "Generate full-featured bash CLIs from a config.", "https://bashly.dev/"),
					repo("bash-it", "Community framework of aliases and plugins.", "https://github.com/bash-it/bash-it"),
					repo("yq", "Like jq, for YAML, XML, and TOML.", "https://github.com/mikefarah/yq"),
					repo("ajv-cli", "Validate JSON against JSON Schema from the CLI.", "https://github.com/ajv-validator/ajv-cli"),
					repo("bats-core", "TAP-compliant testing for bash scripts.", "https://github.com/bats-core/bats-core"),
				},
			},
		},
	},
	{
		Slug: "rust",
		Name: "Rust",
		Logo: "rust",
		Desc: "Bindings, blockchain stacks, and wallet cryptography.",
		Sections: []librarySection{
			{
				Label: "Rust library",
				Blurb: "Ecosystem picks from tooling to on-chain wallets.",
				Tag:   "01 / BUILD",
				Items: []link{
					repo("ts-rs", "Generate TypeScript bindings from Rust structs.", "https://github.com/Aleph-Alpha/ts-rs"),
					repo("awesome-rust", "Curated list of Rust libraries and tools.", "https://github.com/uhub/awesome-rust"),
					repo("ethers-rs", "Ethereum wallet, ABI, and contract toolkit.", "https://github.com/gakonst/ethers-rs"),
					repo("alloy", "Modular Rust stack for Ethereum apps.", "https://github.com/alloy-rs/alloy"),
					repo("solana", "Solana blockchain monorepo and SDKs.", "https://github.com/solana-labs/solana"),
					repo("bip39-rs", "BIP-39 mnemonic generation and recovery.", "https://github.com/maciejhirsz/bip39-rs"),
					repo("hdwallet", "BIP-32 hierarchical deterministic wallet keys.", "https://github.com/k256/hdwallet"),
				},
			},
		},
	},
}

func libraryShelfBySlug(slug string) (libraryShelf, bool) {
	for _, s := range libraryShelves {
		if s.Slug == slug {
			return s, true
		}
	}
	return libraryShelf{}, false
}

// shelfToGroup converts one library section into the shared card group markup.
func shelfToGroup(shelf libraryShelf, section librarySection) group {
	return group{
		Key:   "library-" + shelf.Slug,
		Label: section.Label,
		Tag:   section.Tag,
		Blurb: section.Blurb,
		Items: section.Items,
	}
}

func handleLibrary(w http.ResponseWriter, r *http.Request) {
	items := make([]link, 0, len(libraryShelves)+1)
	for _, s := range libraryShelves {
		items = append(items, view(s.Name, s.Desc, "/library/"+s.Slug, s.Logo))
	}
	items = append(items, view("Browse library", "Every shelf in the archive.", "/library", ""))
	Render(w, "catalog_page", pageData{
		Title: "Library",
		Page:  "library",
		Nav:   nav(),
		Group: group{
			Key:   "library",
			Label: "Library",
			Tag:   "02 / READ",
			Blurb: "Pick a language, then browse the shelves kept for it.",
			Items: items,
		},
	})
}

func handleLibraryShelf(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	shelf, ok := libraryShelfBySlug(slug)
	if !ok {
		notFound(w)
		return
	}
	groups := make([]group, 0, len(shelf.Sections))
	for _, sec := range shelf.Sections {
		groups = append(groups, shelfToGroup(shelf, sec))
	}
	Render(w, "library_page", pageData{
		Title:  shelf.Name + " Library",
		Page:   "library",
		Trail:  "Library",
		Nav:    nav(),
		Group:  group{Key: "library", Label: "Library", Tag: "02 / READ", Blurb: "Pick a language, then browse the shelves kept for it."},
		Groups: groups,
		Doc:    &docView{Slug: shelf.Slug, Label: shelf.Name, Logo: shelf.Logo, Desc: shelf.Desc},
	})
}
