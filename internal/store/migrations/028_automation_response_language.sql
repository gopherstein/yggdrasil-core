-- The language an automation's results are written in (multilingual spec
-- §22): NULL or "account" follows the assistant language setting, "app" the
-- App language, "auto" the language of the automation's request, or a BCP
-- 47 tag such as "de".
ALTER TABLE automations ADD COLUMN response_language TEXT;
