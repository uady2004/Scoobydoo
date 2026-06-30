import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const _kKey = 'app_theme_mode';

// Populated in main() before runApp so build() returns the persisted value
// with no flicker.
ThemeMode initialThemeMode = ThemeMode.dark;

class ThemeNotifier extends Notifier<ThemeMode> {
  @override
  ThemeMode build() => initialThemeMode;

  void setMode(ThemeMode mode) {
    state = mode;
    SharedPreferences.getInstance().then(
      (prefs) => prefs.setString(_kKey, _encode(mode)),
    );
  }

  static ThemeMode decode(String? s) => switch (s) {
        'light' => ThemeMode.light,
        'system' => ThemeMode.system,
        _ => ThemeMode.dark,
      };

  static String _encode(ThemeMode m) => switch (m) {
        ThemeMode.light => 'light',
        ThemeMode.system => 'system',
        ThemeMode.dark => 'dark',
      };
}

final themeProvider = NotifierProvider<ThemeNotifier, ThemeMode>(
  ThemeNotifier.new,
);
