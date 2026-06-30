import 'package:flutter/material.dart';

/// Central color palette for the TikTok clone.
/// All colors are defined here and referenced throughout the app.
/// Never hardcode colors in widgets — always use AppColors.
abstract final class AppColors {
  // ── Brand ──────────────────────────────────────────────────────────────────
  static const Color primary = Color(0xFFEE1D52);
  static const Color secondary = Color(0xFF69C9D0);

  // ── Backgrounds ────────────────────────────────────────────────────────────
  static const Color background = Color(0xFF000000);
  static const Color surface = Color(0xFF161823);
  static const Color surfaceVariant = Color(0xFF2A2A2A);

  // ── Text ───────────────────────────────────────────────────────────────────
  static const Color textPrimary = Colors.white;
  static const Color textSecondary = Color(0xFF8A8B91);
  static const Color textTertiary = Color(0xFF545460);

  // ── Borders & Dividers ─────────────────────────────────────────────────────
  static const Color divider = Color(0xFF2A2A2A);

  // ── Semantic ───────────────────────────────────────────────────────────────
  static const Color error = Color(0xFFFF3040);
  static const Color success = Color(0xFF25F4EE);
  static const Color warning = Color(0xFFFFB800);

  // ── Gradients ──────────────────────────────────────────────────────────────
  static const LinearGradient gradient = LinearGradient(
    colors: [Color(0xFFEE1D52), Color(0xFFFF7A7A)],
  );

  static const LinearGradient gradientTeal = LinearGradient(
    colors: [Color(0xFF25F4EE), Color(0xFF69C9D0)],
  );

  static const LinearGradient gradientDark = LinearGradient(
    begin: Alignment.topCenter,
    end: Alignment.bottomCenter,
    colors: [Colors.transparent, Color(0xCC000000)],
  );

  static const LinearGradient gradientBackground = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [Color(0xFF161823), Color(0xFF000000)],
  );

  // ── Overlay ────────────────────────────────────────────────────────────────
  static const Color overlayLight = Color(0x1AFFFFFF);
  static const Color overlayDark = Color(0x80000000);

  // ── Interaction states ─────────────────────────────────────────────────────
  static const Color like = Color(0xFFEE1D52);
  static const Color verified = Color(0xFF20D5EC);
  static const Color online = Color(0xFF44D62C);
  static const Color live = Color(0xFFFF0050);
}
