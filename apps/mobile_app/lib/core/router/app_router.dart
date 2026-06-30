import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

// ── Auth
import '../../features/auth/presentation/screens/login_screen.dart';
import '../../features/auth/presentation/screens/register_screen.dart';
import '../../features/auth/presentation/screens/otp_screen.dart';
import '../../features/auth/presentation/screens/forgot_password_screen.dart';

// ── Shell
import '../../shared/widgets/main_shell.dart';

// ── Main tabs
import '../../features/home_feed/presentation/screens/home_screen.dart';
import '../../features/search/presentation/screens/search_screen.dart';
import '../../features/upload_video/presentation/screens/upload_screen.dart';
import '../../features/messaging/presentation/screens/inbox_screen.dart';
import '../../features/messaging/presentation/screens/chat_screen.dart';
import '../../features/profile/presentation/screens/profile_screen.dart';

// ── Profile / social
import '../../features/followers/presentation/screens/followers_screen.dart';

// ── Video
import '../../features/video_player/presentation/screens/video_player_screen.dart';
import '../../features/comments/presentation/screens/comments_screen.dart';

// ── Notifications / Settings
import '../../features/notifications/notification_screen.dart';
import '../../features/settings/settings_screen.dart';
import '../../features/settings/privacy_settings_screen.dart';
import '../../features/settings/notification_settings_screen.dart';
import '../../features/settings/security_screen.dart';

// ── Discovery
import '../../features/hashtags/presentation/screens/hashtag_screen.dart';
import '../../features/sounds/presentation/screens/sound_screen.dart';

// ── Commerce
import '../../features/wallet/presentation/screens/wallet_screen.dart';
import '../../features/wallet/presentation/screens/buy_coins_screen.dart';
import '../../features/ecommerce/presentation/screens/shop_screen.dart';
import '../../features/ecommerce/presentation/screens/product_screen.dart';
import '../../features/orders/presentation/screens/cart_screen.dart';
import '../../features/orders/presentation/screens/checkout_screen.dart';
import '../../features/orders/presentation/screens/orders_screen.dart';
import '../../features/orders/presentation/screens/order_detail_screen.dart';

// ── Live
import '../../features/livestream/presentation/screens/live_viewer_screen.dart';
import '../../features/livestream/presentation/screens/go_live_screen.dart';

// ── Creator
import '../../features/creator_dashboard/analytics_screen.dart';
import '../../features/creator_dashboard/dashboard_screen.dart';

// ── Report
import '../../features/report/report_screen.dart';

// ── Edit profile
import '../../features/profile/presentation/screens/edit_profile_screen.dart';

// ── Search results
import '../../features/search/presentation/screens/search_results_screen.dart';

// ── Notification preferences
import '../../features/notifications/notification_preferences_screen.dart';

// ── Creator profile
import '../../features/creator_profile/presentation/screens/creator_profile_screen.dart';
import '../../features/profile/domain/entities/profile_entity.dart';
final appRouterProvider = Provider<GoRouter>((ref) {
  return GoRouter(
    initialLocation: '/login',
    debugLogDiagnostics: false,
    routes: [
      // ── Auth ────────────────────────────────────────────────────────────────
      GoRoute(
        path: '/login',
        name: 'login',
        builder: (c, s) => const LoginScreen(),
      ),
      GoRoute(
        path: '/register',
        name: 'register',
        builder: (c, s) => const RegisterScreen(),
      ),
      GoRoute(
        path: '/otp',
        name: 'otp',
        builder: (c, s) =>
            OtpScreen(phone: s.uri.queryParameters['phone'] ?? ''),
      ),
      GoRoute(
        path: '/forgot-password',
        name: 'forgot-password',
        builder: (c, s) => const ForgotPasswordScreen(),
      ),

      // ── Main shell (indexed tabs) ────────────────────────────────────────────
      StatefulShellRoute.indexedStack(
        builder: (c, s, shell) => MainShell(navigationShell: shell),
        branches: [
          StatefulShellBranch(routes: [
            GoRoute(
              path: '/home',
              name: 'home',
              builder: (c, s) => const HomeScreen(),
            ),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(
              path: '/search',
              name: 'search',
              builder: (c, s) => const SearchScreen(),
            ),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(
              path: '/upload',
              name: 'upload',
              builder: (c, s) => const UploadScreen(),
            ),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(
              path: '/inbox',
              name: 'inbox',
              builder: (c, s) => const InboxScreen(),
              routes: [
                GoRoute(
                  path: '/chat/:conversationId',
                  name: 'chat',
                  builder: (c, s) => ChatScreen(
                    conversationId: s.pathParameters['conversationId']!,
                  ),
                ),
              ],
            ),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(
              path: '/me',
              name: 'me',
              builder: (c, s) => const ProfileScreen(),
            ),
          ]),
        ],
      ),

      // ── Profile ──────────────────────────────────────────────────────────────
      GoRoute(
        path: '/profile/:username',
        name: 'profile-username',
        builder: (c, s) => ProfileScreen(userId: s.pathParameters['username']),
      ),
      GoRoute(
        path: '/edit-profile',
        name: 'edit-profile',
        builder: (c, s) => EditProfileScreen(
          profile: s.extra as ProfileEntity?,
        ),
      ),
      GoRoute(
        path: '/creator/:userId',
        name: 'creator-profile',
        builder: (c, s) =>
            CreatorProfileScreen(userId: s.pathParameters['userId']),
      ),

      // ── Video ────────────────────────────────────────────────────────────────
      GoRoute(
        path: '/video/:videoId',
        name: 'video-detail',
        builder: (c, s) => VideoPlayerScreen(
          videoId: s.pathParameters['videoId']!,
        ),
      ),
      
      GoRoute(
        path: '/comments/:videoId',
        name: 'comments',
        builder: (c, s) =>
            CommentsScreen(videoId: s.pathParameters['videoId']!),
      ),

      // ── Notifications / settings ─────────────────────────────────────────────
      GoRoute(
        path: '/notifications',
        name: 'notifications',
        builder: (c, s) => const NotificationScreen(),
      ),
      GoRoute(
        path: '/settings',
        name: 'settings',
        builder: (c, s) => const SettingsScreen(),
      ),
      GoRoute(
        path: '/settings/privacy',
        name: 'settings-privacy',
        builder: (c, s) => const PrivacySettingsScreen(),
      ),
      GoRoute(
        path: '/settings/notifications',
        name: 'settings-notifications',
        builder: (c, s) => const NotificationSettingsScreen(),
      ),
      GoRoute(
        path: '/settings/security',
        name: 'settings-security',
        builder: (c, s) => const SecurityScreen(),
      ),
      GoRoute(
        path: '/settings/notification-preferences',
        name: 'settings-notification-preferences',
        builder: (c, s) => const NotificationPreferencesScreen(),
      ),

      // ── Social / discovery ───────────────────────────────────────────────────
      GoRoute(
        path: '/hashtag/:tag',
        name: 'hashtag',
        builder: (c, s) => HashtagScreen(tag: s.pathParameters['tag']!),
      ),
      GoRoute(
        path: '/sound/:id',
        name: 'sound',
        builder: (c, s) => SoundScreen(soundId: s.pathParameters['id']!),
      ),
      GoRoute(
        path: '/followers/:userId',
        name: 'followers',
        builder: (c, s) => FollowersScreen(
          userId: s.pathParameters['userId']!,
        ),
      ),
      GoRoute(
        path: '/following/:userId',
        name: 'following',
        builder: (c, s) => FollowersScreen(
          userId: s.pathParameters['userId']!,
        ),
      ),
      GoRoute(
        path: '/search/results',
        name: 'search-results',
        builder: (c, s) => SearchResultsScreen(
          query: s.uri.queryParameters['q'] ?? '',
          type: s.uri.queryParameters['type'],
        ),
      ),

      // ── Commerce ─────────────────────────────────────────────────────────────
      GoRoute(
        path: '/wallet',
        name: 'wallet',
        builder: (c, s) => const WalletScreen(),
      ),
      GoRoute(
        path: '/buy-coins',
        name: 'buy-coins',
        builder: (c, s) => const BuyCoinsScreen(),
      ),
      GoRoute(
        path: '/shop',
        name: 'shop',
        builder: (c, s) => const ShopScreen(),
      ),
      GoRoute(
        path: '/product/:id',
        name: 'product',
        builder: (c, s) => ProductScreen(productId: s.pathParameters['id']!),
      ),
      GoRoute(
        path: '/cart',
        name: 'cart',
        builder: (c, s) => const CartScreen(),
      ),
      GoRoute(
        path: '/checkout',
        name: 'checkout',
        builder: (c, s) => const CheckoutScreen(),
      ),
      GoRoute(
        path: '/orders',
        name: 'orders',
        builder: (c, s) => const OrdersScreen(),
      ),
      GoRoute(
        path: '/orders/:id',
        name: 'order-detail',
        builder: (c, s) => OrderDetailScreen(orderId: s.pathParameters['id']!),
      ),

      // ── Live ─────────────────────────────────────────────────────────────────
      GoRoute(
        path: '/live/:streamId',
        name: 'live',
        builder: (c, s) =>
            LiveViewerScreen(streamId: s.pathParameters['streamId']!),
      ),
      GoRoute(
        path: '/go-live',
        name: 'go-live',
        builder: (c, s) => const GoLiveScreen(),
      ),

      // ── Creator ──────────────────────────────────────────────────────────────
      GoRoute(
        path: '/analytics',
        name: 'analytics',
        builder: (c, s) => const AnalyticsScreen(),
      ),
      GoRoute(
        path: '/dashboard',
        name: 'dashboard',
        builder: (c, s) => const DashboardScreen(),
      ),

      // ── Report ───────────────────────────────────────────────────────────────
      GoRoute(
        path: '/report',
        name: 'report',
        builder: (c, s) => ReportScreen(
          contentId: s.uri.queryParameters['id']!,
          contentType: s.uri.queryParameters['type']!,
        ),
      ),
    ],
  );
});
