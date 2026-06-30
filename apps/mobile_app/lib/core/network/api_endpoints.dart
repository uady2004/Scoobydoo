// ignore_for_file: constant_identifier_names

import 'package:flutter/foundation.dart' show kIsWeb;

/// Central registry of every API endpoint used by the app.
/// All paths that contain a path parameter are expressed as
/// static methods so the parameter is interpolated at the call
/// site, keeping every URL in one place.
///
/// Base URL is configurable at build time via:
///   flutter run --dart-define=API_BASE_URL=http://192.168.1.x:8000/api/v1
abstract final class ApiEndpoints {
  // ── Base ────────────────────────────────────────────────────
  // Picks the right host automatically:
  //   Flutter Web  → localhost (browser runs on host machine)
  //   Android emulator → 10.0.2.2 (loopback alias to host)
  //   Override     → set API_BASE_URL at build time

  // static String get baseUrl {
  //   const envUrl = String.fromEnvironment('API_BASE_URL');
  //   if (envUrl.isNotEmpty) return envUrl;
  //   if (kIsWeb) return 'http://localhost:8000/api/v1';
  //   return 'http://10.0.2.2:8000/api/v1';
  // }

static String get baseUrl {
  const envUrl = String.fromEnvironment('API_BASE_URL');
  if (envUrl.isNotEmpty) return envUrl;
  if (kIsWeb) return 'https://scoobydoo.onrender.com';
  return 'https://scoobydoo.onrender.com';
}

  // ── Auth ────────────────────────────────────────────────────
  static const String login = '/auth/login';
  static const String register = '/auth/register';
  static const String logout = '/auth/logout';
  static const String refresh = '/auth/refresh';
  static const String forgotPassword = '/auth/forgot-password';
  static const String resetPassword = '/auth/reset-password';
  static const String googleAuth = '/auth/oauth/google';
  static const String appleAuth = '/auth/oauth/apple';
  // OTP — routed through user-service
  static const String otpSend = '/auth/otp/send';
  static const String otpVerify = '/auth/otp/verify';
  static const String verifyEmail = '/auth/verify-email';

  // ── Users ───────────────────────────────────────────────────
  static const String me = '/users/me';
  static String userById(String id) => '/users/$id';
  // No /profile suffix — the user object contains all profile fields
  static String userProfile(String id) => '/users/$id';
  static String followUser(String id) => '/users/$id/follow';
  // Backend uses DELETE on the same /follow path for unfollow
  static String unfollowUser(String id) => '/users/$id/follow';
  static String userFollowers(String id) => '/users/$id/followers';
  static String userFollowing(String id) => '/users/$id/following';
  static String userVideos(String id) => '/users/$id/videos';

  // ── Videos ──────────────────────────────────────────────────
  static const String videos = '/videos';
  static String videoById(String id) => '/videos/$id';
  static const String videoUploadInit = '/videos/upload/init';
  static const String videoUploadChunk = '/videos/upload/chunk';
  static const String videoUploadComplete = '/videos/upload/complete';
  static String videoView(String id) => '/videos/$id/view';
  static String videoComments(String id) => '/videos/$id/comments';

  // ── Feed ────────────────────────────────────────────────────
  static const String feed = '/feed';
  static const String feedForYou = '/feed/for-you';
  static const String feedFollowing = '/feed/following';
  static const String feedTrending = '/videos/trending';
  static const String feedView = '/feed/view';

  // ── Interactions ────────────────────────────────────────────
  static String videoLike(String id) => '/videos/$id/like';
  static String videoLikeStatus(String id) => '/videos/$id/like-status';
  static String videoBookmark(String id) => '/videos/$id/bookmark';
  static String commentLike(String id) => '/comments/$id/like';
  static String commentPin(String id) => '/comments/$id/pin';
  static String commentReport(String id) => '/comments/$id/report';
  static const String likedVideos = '/me/liked-videos';
  static const String bookmarks = '/me/bookmarks';
  static const String bookmarkCollections = '/bookmarks/collections';

  // ── Search ──────────────────────────────────────────────────
  static const String search = '/search';
  static const String searchTrending = '/search/trending';
  static const String searchSuggestions = '/search/suggestions';
  static const String searchHistory = '/search/history';

  // ── Sounds ──────────────────────────────────────────────────
  static String soundById(String id) => '/sounds/$id';
  static const String soundsTrending = '/sounds/trending';
  static const String soundsSearch = '/sounds/search';

  // ── Hashtags ────────────────────────────────────────────────
  static String hashtagByTag(String tag) => '/hashtags/$tag';
  static const String hashtagsTrending = '/hashtags/trending';

  // ── Messaging ───────────────────────────────────────────────
  static const String conversations = '/conversations';
  static String conversationMessages(String id) => '/conversations/$id/messages';
  static String conversationRead(String id) => '/conversations/$id/read';
  static const String conversationGroup = '/conversations/group';
  static const String messageMedia = '/messages/media';

  // ── Notifications ───────────────────────────────────────────
  static const String notifications = '/notifications';
  static String notificationRead(String id) => '/notifications/$id/read';
  static const String notificationsReadAll = '/notifications/read-all';
  static const String notificationsUnreadCount = '/notifications/unread-count';

  // ── Livestream ──────────────────────────────────────────────
  static const String streams = '/live';
  static String streamById(String id) => '/live/$id';
  static String streamViewers(String id) => '/live/$id/viewers';
  static String streamStop(String id) => '/live/$id/stop';
  static const String liveStart = '/live/start';
  static const String gifts = '/gifts';
  static const String giftSend = '/gifts/send';

  // ── Wallet ──────────────────────────────────────────────────
  static const String walletBalance = '/wallet/balance';
  static const String walletPackages = '/wallet/packages';
  static const String walletTransactions = '/wallet/transactions';
  static const String walletCoinsBuy = '/wallet/coins/buy';
  static const String walletCoinsConfirm = '/wallet/coins/confirm';
  static const String walletWithdraw = '/wallet/withdraw';

  // ── Ecommerce ───────────────────────────────────────────────
  static const String products = '/products';
  static String productById(String id) => '/products/$id';
  static const String productsSearch = '/products/search';
  static const String cart = '/cart';
  static const String cartItems = '/cart/items';
  static String cartItem(String id) => '/cart/items/$id';
  static const String orders = '/orders';
  static String orderById(String id) => '/orders/$id';
  static String orderCancel(String id) => '/orders/$id/cancel';

  // ── Analytics ───────────────────────────────────────────────
  static const String analyticsCreator = '/analytics/profile/stats';
  static String analyticsVideo(String id) => '/analytics/videos/$id/stats';
  static const String analyticsRevenue = '/analytics/revenue';

  // ── Reports ─────────────────────────────────────────────────
  static const String reports = '/reports';
}
