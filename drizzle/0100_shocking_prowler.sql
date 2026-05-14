CREATE TABLE IF NOT EXISTS "accounting_new_api_configs" (
	"id" serial PRIMARY KEY NOT NULL,
	"name" varchar(64) DEFAULT 'default' NOT NULL,
	"base_url" varchar(512) NOT NULL,
	"access_token" text NOT NULL,
	"admin_user_id" integer NOT NULL,
	"page_size" integer DEFAULT 100 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now(),
	"updated_at" timestamp with time zone DEFAULT now(),
	CONSTRAINT "accounting_new_api_configs_name_unique" UNIQUE("name")
);
