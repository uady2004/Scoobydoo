class AppConstants {
  // API base URL — override via env or a build flavor.
  static const String apiUrl = 'http://10.0.2.2:8000/api/v1';

  // Secure-storage keys.
  static const String token = 'access_token';
  static const String refreshToken = 'refresh_token';
  static const String userId = 'user_id';

  // App metadata.
  static const String appName = 'TikTok Clone';
  static const String appVersion = '1.0.0';

  // Pagination defaults.
  static const int defaultPageLimit = 20;
}
