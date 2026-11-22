-- The Provider of each Account. See GLOSSARY.md, "Provider".
-- Each Account before this migration came from an XTB export. Thus the default is 'xtb'.
ALTER TABLE accounts ADD COLUMN provider TEXT NOT NULL DEFAULT 'xtb' CHECK (provider IN ('xtb', 'kraken'));
