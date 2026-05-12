import { ExternalLink, Info, X } from "lucide-react";
import { useEffect } from "react";

import {
  APP_BUILD,
  APP_COPYRIGHT,
  APP_LICENSE,
  APP_LICENSE_URL,
  APP_RELEASES_URL,
  APP_REPOSITORY_URL,
  APP_VERSION,
} from "../appInfo";
import { useI18n } from "../i18n/I18nProvider";

export type AboutDialogProps = {
  open: boolean;
  onClose: () => void;
};

export function AboutDialog({ open, onClose }: AboutDialogProps) {
  const { t } = useI18n();

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="about-modal-layer">
      <button
        type="button"
        className="about-modal-backdrop"
        onClick={onClose}
        aria-label={t("aboutClose")}
      />
      <section
        className="about-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="about-dialog-title"
      >
        <header className="about-dialog-header">
          <span className="about-dialog-mark" aria-hidden="true">
            <Info className="about-dialog-icon" />
          </span>
          <div className="about-dialog-title-block">
            <h2 id="about-dialog-title">{t("aboutTitle")}</h2>
            <p>{t("aboutDescription")}</p>
          </div>
          <button
            type="button"
            className="about-dialog-close"
            onClick={onClose}
            aria-label={t("aboutClose")}
          >
            <X className="nav-lucide" />
          </button>
        </header>

        <dl className="about-meta">
          <div className="about-meta-row">
            <dt>{t("aboutVersion")}</dt>
            <dd>{APP_VERSION}</dd>
          </div>
          <div className="about-meta-row">
            <dt>{t("aboutBuild")}</dt>
            <dd>{APP_BUILD}</dd>
          </div>
          <div className="about-meta-row">
            <dt>{t("aboutCopyright")}</dt>
            <dd>{APP_COPYRIGHT}</dd>
          </div>
          <div className="about-meta-row">
            <dt>{t("aboutLicense")}</dt>
            <dd>{APP_LICENSE}</dd>
          </div>
        </dl>

        <div className="about-actions">
          <a href={APP_LICENSE_URL} target="_blank" rel="noreferrer" className="about-link">
            {t("aboutLicense")}
            <ExternalLink className="nav-lucide-sm" />
          </a>
          <a href={APP_REPOSITORY_URL} target="_blank" rel="noreferrer" className="about-link">
            {t("aboutRepository")}
            <ExternalLink className="nav-lucide-sm" />
          </a>
          <a href={APP_RELEASES_URL} target="_blank" rel="noreferrer" className="about-link">
            {t("aboutReleaseNotes")}
            <ExternalLink className="nav-lucide-sm" />
          </a>
        </div>
      </section>
    </div>
  );
}
