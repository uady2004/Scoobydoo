import 'package:flutter/material.dart';

import 'app_colors.dart';

/// Typography scale for the TikTok clone.
///
/// Uses the system font stack — no Google Fonts dependency required.
/// Every style defaults to [AppColors.textPrimary] (white) so widgets
/// don't need to override the color for the common case.
///
/// Scale reference:
///   display   32 / bold    — hero numbers, splash text
///   headline  20 / w700    — section titles, creator name on profile
///   title     17 / w600    — card titles, bottom-sheet headers
///   body      14 / w400    — captions, descriptions, comment text
///   caption   12 / w400    — timestamps, counts, metadata
///   tiny      10 / w400    — badges, labels on dense UI
abstract final class AppTextStyles {
  // ── Display ────────────────────────────────────────────────────────────────

  /// 32px bold — hero text, large stat numbers.
  static const TextStyle display = TextStyle(
    fontSize: 32,
    fontWeight: FontWeight.bold,
    color: AppColors.textPrimary,
    letterSpacing: -0.5,
    height: 1.1,
  );

  /// Display with brand-primary color — use for numbers that need emphasis.
  static const TextStyle displayPrimary = TextStyle(
    fontSize: 32,
    fontWeight: FontWeight.bold,
    color: AppColors.primary,
    letterSpacing: -0.5,
    height: 1.1,
  );

  // ── Headline ───────────────────────────────────────────────────────────────

  /// 20px w700 — section titles, screen headings.
  static const TextStyle headline = TextStyle(
    fontSize: 20,
    fontWeight: FontWeight.w700,
    color: AppColors.textPrimary,
    letterSpacing: -0.2,
    height: 1.3,
  );

  /// 20px w700 secondary — muted section labels.
  static const TextStyle headlineSecondary = TextStyle(
    fontSize: 20,
    fontWeight: FontWeight.w700,
    color: AppColors.textSecondary,
    letterSpacing: -0.2,
    height: 1.3,
  );

  // ── Title ──────────────────────────────────────────────────────────────────

  /// 17px w600 — card titles, sheet headers, tab labels.
  static const TextStyle title = TextStyle(
    fontSize: 17,
    fontWeight: FontWeight.w600,
    color: AppColors.textPrimary,
    letterSpacing: 0,
    height: 1.35,
  );

  /// 17px w600 secondary — de-emphasised titles.
  static const TextStyle titleSecondary = TextStyle(
    fontSize: 17,
    fontWeight: FontWeight.w600,
    color: AppColors.textSecondary,
    letterSpacing: 0,
    height: 1.35,
  );

  // ── Body ───────────────────────────────────────────────────────────────────

  /// 14px w400 — standard body copy, captions on video, comment text.
  static const TextStyle body = TextStyle(
    fontSize: 14,
    fontWeight: FontWeight.w400,
    color: AppColors.textPrimary,
    letterSpacing: 0,
    height: 1.5,
  );

  /// 14px w600 — bold body, usernames in comment threads.
  static const TextStyle bodySemibold = TextStyle(
    fontSize: 14,
    fontWeight: FontWeight.w600,
    color: AppColors.textPrimary,
    letterSpacing: 0,
    height: 1.5,
  );

  /// 14px w400 secondary — muted body text.
  static const TextStyle bodySecondary = TextStyle(
    fontSize: 14,
    fontWeight: FontWeight.w400,
    color: AppColors.textSecondary,
    letterSpacing: 0,
    height: 1.5,
  );

  /// 14px w400 teal — hashtag / mention style.
  static const TextStyle bodyTeal = TextStyle(
    fontSize: 14,
    fontWeight: FontWeight.w400,
    color: AppColors.secondary,
    letterSpacing: 0,
    height: 1.5,
  );

  // ── Caption ────────────────────────────────────────────────────────────────

  /// 12px w400 — timestamps, stat counts, metadata labels.
  static const TextStyle caption = TextStyle(
    fontSize: 12,
    fontWeight: FontWeight.w400,
    color: AppColors.textPrimary,
    letterSpacing: 0,
    height: 1.4,
  );

  /// 12px w400 secondary — de-emphasised captions.
  static const TextStyle captionSecondary = TextStyle(
    fontSize: 12,
    fontWeight: FontWeight.w400,
    color: AppColors.textSecondary,
    letterSpacing: 0,
    height: 1.4,
  );

  /// 12px w600 — small-but-important labels (e.g. LIVE badge text).
  static const TextStyle captionSemibold = TextStyle(
    fontSize: 12,
    fontWeight: FontWeight.w600,
    color: AppColors.textPrimary,
    letterSpacing: 0.2,
    height: 1.4,
  );

  // ── Tiny ───────────────────────────────────────────────────────────────────

  /// 10px w400 — dense badges, overlay counters.
  static const TextStyle tiny = TextStyle(
    fontSize: 10,
    fontWeight: FontWeight.w400,
    color: AppColors.textPrimary,
    letterSpacing: 0,
    height: 1.4,
  );

  /// 10px w600 — bold micro-labels.
  static const TextStyle tinySemibold = TextStyle(
    fontSize: 10,
    fontWeight: FontWeight.w600,
    color: AppColors.textPrimary,
    letterSpacing: 0.5,
    height: 1.4,
  );

  /// 10px w400 secondary.
  static const TextStyle tinySecondary = TextStyle(
    fontSize: 10,
    fontWeight: FontWeight.w400,
    color: AppColors.textSecondary,
    letterSpacing: 0,
    height: 1.4,
  );
}
