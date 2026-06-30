import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/livestream_model.dart';
import '../../providers/livestream_provider.dart';
import '../../repositories/livestream_repository.dart';
import '../../services/livestream_websocket_service.dart';
import '../../widgets/gift_animation_overlay.dart';
import '../../widgets/live_chat_overlay.dart';
import '../../widgets/poll_overlay.dart';
import '../../widgets/viewer_count_badge.dart';
import '../../../../core/network/api_client.dart';
import '../../../gifts/data/models/gift_model.dart';
import '../../../gifts/presentation/screens/gift_sheet.dart';

// ---------------------------------------------------------------------------
// Floating heart animation data
// ---------------------------------------------------------------------------

class _FloatingHeart {
  _FloatingHeart({
    required this.id,
    required this.color,
    required this.xFraction,
  });

  final int id;
  final Color color;
  final double xFraction; // 0..1 horizontal position
  double progress = 0.0; // 0 = bottom, 1 = fully floated away
}

class LiveViewerScreen extends ConsumerStatefulWidget {
  const LiveViewerScreen({super.key, required this.streamId});

  final String streamId;

  @override
  ConsumerState<LiveViewerScreen> createState() => _LiveViewerScreenState();
}

class _LiveViewerScreenState extends ConsumerState<LiveViewerScreen> {
  late final LivestreamViewerProvider _provider;
  bool _isFollowing = false;
  bool _controlsVisible = true;

  // Mock data for dev
  bool _isMockLoaded = false;
  int _mockViewerCount = 0;
  String _mockStreamerName = 'Live Creator';
  Timer? _viewerSimTimer;

  // Floating hearts
  final List<_FloatingHeart> _hearts = [];
  int _heartIdCounter = 0;
  Timer? _heartAnimTimer;

  static const _heartColors = [
    Color(0xFFFF2D55),
    Color(0xFFFF6B8A),
    Color(0xFFFF9500),
    Color(0xFFFFD700),
    Color(0xFF69C9D0),
    Color(0xFF9B59B6),
  ];

  @override
  void initState() {
    super.initState();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    _provider = LivestreamViewerProvider(
      repo: LivestreamRepository(dio: ApiClient.instance.dio),
      wsService: LivestreamWebSocketService(
        wsBaseUrl: 'wss://api.tiktokclone.com/ws',
        authToken: '',
      ),
    );
    _provider.addListener(_onProviderUpdate);
    _loadMockStream();
    _startHeartAnimation();
  }

  void _loadMockStream() {
    Future.delayed(const Duration(milliseconds: 800), () {
      if (!mounted) return;
      setState(() {
        _isMockLoaded = true;
        _mockViewerCount = 8400;
        _mockStreamerName = widget.streamId.contains('_')
            ? widget.streamId.split('_').first
            : '@live_creator';
      });

      // Simulate viewer count fluctuating
      _viewerSimTimer =
          Timer.periodic(const Duration(seconds: 5), (_) {
        if (!mounted) return;
        setState(() {
          final delta = (math.Random().nextInt(40) - 10);
          _mockViewerCount = (_mockViewerCount + delta).clamp(100, 999999);
        });
      });
    });
  }

  void _startHeartAnimation() {
    _heartAnimTimer =
        Timer.periodic(const Duration(milliseconds: 50), (_) {
      if (!mounted) return;
      setState(() {
        for (var i = _hearts.length - 1; i >= 0; i--) {
          _hearts[i].progress += 0.018;
          if (_hearts[i].progress >= 1.0) _hearts.removeAt(i);
        }
      });
    });
  }

  void _spawnHeart() {
    final rand = math.Random();
    setState(() {
      _hearts.add(_FloatingHeart(
        id: _heartIdCounter++,
        color: _heartColors[rand.nextInt(_heartColors.length)],
        xFraction: 0.6 + rand.nextDouble() * 0.3,
      ));
    });
  }

  void _onProviderUpdate() {
    if (!mounted) return;
    setState(() {});
  }

  @override
  void dispose() {
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    _provider.removeListener(_onProviderUpdate);
    _provider.leaveStream();
    _provider.dispose();
    _viewerSimTimer?.cancel();
    _heartAnimTimer?.cancel();
    super.dispose();
  }

  void _showTopUpSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: Colors.white,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => const _TopUpSheet(),
    );
  }

  Future<void> _sendGift(GiftModel gift, int quantity) async {
    await _provider.sendGift(gift.id, quantity: quantity);
  }

  int get _displayViewerCount =>
      _provider.viewerCount > 0
          ? _provider.viewerCount
          : (_isMockLoaded ? _mockViewerCount : 0);

  String get _displayStreamerName =>
      _provider.stream?.hostUsername ?? _mockStreamerName;

  @override
  Widget build(BuildContext context) {
    final stream = _provider.stream;
    final isEnded = stream?.status == StreamStatus.ended;

    return Scaffold(
      backgroundColor: Colors.black,
      body: GestureDetector(
        onTap: () => setState(() => _controlsVisible = !_controlsVisible),
        onDoubleTap: () {
          // Double-tap anywhere to send a heart
          for (var i = 0; i < 3; i++) {
            Future.delayed(Duration(milliseconds: i * 80), _spawnHeart);
          }
        },
        child: Stack(
          fit: StackFit.expand,
          children: [
            // Video / background layer
            _buildVideoLayer(),

            // Stream-ended overlay
            if (isEnded) _buildEndedOverlay(),

            // Loading
            if (_provider.loading && stream == null && !_isMockLoaded)
              const Center(
                child: CircularProgressIndicator(
                    color: Color(0xFFFF2D55)),
              ),

            // Top bar
            if (!isEnded)
              AnimatedOpacity(
                opacity: _controlsVisible ? 1.0 : 0.0,
                duration: const Duration(milliseconds: 200),
                child: _buildTopBar(stream),
              ),

            // Chat overlay
            if (!isEnded)
              Positioned(
                bottom: 88,
                left: 0,
                width: MediaQuery.of(context).size.width * 0.72,
                height: MediaQuery.of(context).size.height * 0.35,
                child: LiveChatOverlay(
                  messages: _provider.messages,
                  pinnedMessage: _provider.pinnedMessage,
                  onSendMessage: _provider.sendMessage,
                  allowComments: stream?.allowComments ?? true,
                ),
              ),

            // Floating hearts layer
            if (!isEnded) _buildFloatingHearts(),

            // Gift animations
            if (_provider.giftQueue.isNotEmpty)
              GiftAnimationOverlay(
                gift: _provider.giftQueue.first,
                onDismiss: _provider.dismissGiftAnimation,
              ),

            // Poll overlay
            if (_provider.activePoll != null)
              Positioned(
                top: 100,
                right: 16,
                width: 180,
                child: PollOverlay(
                  poll: _provider.activePoll!,
                  votedOptionId: _provider.votedOptionId,
                  onVote: (pollId, optionId) =>
                      _provider.votePoll(pollId, optionId),
                  onDismiss: () {},
                ),
              ),

            // Bottom bar
            if (!isEnded)
              Positioned(
                bottom: 0,
                left: 0,
                right: 0,
                child: _buildBottomBar(),
              ),

            // Double-tap hint (shows briefly on first load)
            if (_isMockLoaded)
              Positioned(
                bottom: MediaQuery.of(context).size.height * 0.42,
                right: 20,
                child: const _DoubleTapHint(),
              ),
          ],
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Floating hearts
  // ---------------------------------------------------------------------------

  Widget _buildFloatingHearts() {
    return IgnorePointer(
      child: Stack(
        children: _hearts.map((h) {
          final size = MediaQuery.of(context).size;
          final x = h.xFraction * size.width - 20;
          const bottomY = 100.0;
          final topY = size.height * 0.5;
          final y = bottomY + (1 - h.progress) * (topY - bottomY);

          return Positioned(
            left: x,
            bottom: y,
            child: Opacity(
              opacity: (1.0 - h.progress * 0.8).clamp(0.0, 1.0),
              child: Transform.scale(
                scale: 0.6 + h.progress * 0.6,
                child: Icon(
                  Icons.favorite,
                  color: h.color,
                  size: 28,
                  shadows: const [
                    Shadow(blurRadius: 6, color: Colors.black38),
                  ],
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Video layer
  // ---------------------------------------------------------------------------

  Widget _buildVideoLayer() {
    return Container(
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [Color(0xFF1A0A2E), Color(0xFF0A0A0A)],
        ),
      ),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 90,
              height: 90,
              decoration: const BoxDecoration(
                color: Color(0xFF1E1E2E),
                shape: BoxShape.circle,
              ),
              child: const Icon(Icons.live_tv,
                  color: Color(0xFFFF2D55), size: 44),
            ),
            const SizedBox(height: 16),
            Text(
              _isMockLoaded ? _displayStreamerName : 'Loading...',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 20,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 6),
            const Text(
              'Live Stream',
              style: TextStyle(color: Colors.white38, fontSize: 13),
            ),
          ],
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Top bar
  // ---------------------------------------------------------------------------

  Widget _buildTopBar(LiveStream? stream) {
    return Positioned(
      top: 0,
      left: 0,
      right: 0,
      child: Container(
        padding: EdgeInsets.only(
          top: MediaQuery.of(context).padding.top + 8,
          left: 12,
          right: 12,
          bottom: 12,
        ),
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [
              Colors.black.withValues(alpha: 0.75),
              Colors.transparent,
            ],
          ),
        ),
        child: Row(
          children: [
            // Back
            _CircleButton(
              icon: Icons.arrow_back_ios_new,
              onTap: () => Navigator.pop(context),
            ),
            const SizedBox(width: 10),
            // Streamer avatar
            CircleAvatar(
              radius: 18,
              backgroundColor: const Color(0xFF2A2A2A),
              backgroundImage: (stream?.hostAvatarUrl?.isNotEmpty ?? false)
                  ? NetworkImage(stream!.hostAvatarUrl!)
                  : null,
              child: (stream?.hostAvatarUrl?.isEmpty ?? true)
                  ? const Icon(Icons.person,
                      color: Colors.white, size: 20)
                  : null,
            ),
            const SizedBox(width: 8),
            // Name + title
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    children: [
                      Text(
                        _displayStreamerName,
                        style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.bold,
                          fontSize: 13,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(width: 6),
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: const Color(0xFFFF2D55),
                          borderRadius: BorderRadius.circular(3),
                        ),
                        child: const Text(
                          'LIVE',
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 10,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ],
                  ),
                  if ((stream?.title ?? '').isNotEmpty)
                    Text(
                      stream!.title,
                      style: const TextStyle(
                          color: Colors.white60, fontSize: 11),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            // Viewer count
            ViewerCountBadge(viewerCount: _displayViewerCount),
            const SizedBox(width: 8),
            // Follow button
            GestureDetector(
              onTap: () => setState(() => _isFollowing = !_isFollowing),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),
                padding: const EdgeInsets.symmetric(
                    horizontal: 14, vertical: 6),
                decoration: BoxDecoration(
                  color: _isFollowing
                      ? Colors.transparent
                      : const Color(0xFFFF2D55),
                  border: Border.all(
                    color: _isFollowing
                        ? Colors.white.withValues(alpha: 0.5)
                        : Colors.transparent,
                  ),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Text(
                  _isFollowing ? 'Following' : 'Follow',
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                    fontSize: 12,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Bottom bar
  // ---------------------------------------------------------------------------

  Widget _buildBottomBar() {
    return Container(
      padding: EdgeInsets.fromLTRB(
        12,
        8,
        12,
        MediaQuery.of(context).padding.bottom + 8,
      ),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.bottomCenter,
          end: Alignment.topCenter,
          colors: [
            Colors.black.withValues(alpha: 0.8),
            Colors.transparent,
          ],
        ),
      ),
      child: Row(
        children: [
          // Chat input
          Expanded(
            child: GestureDetector(
              onTap: _showChatInput,
              child: Container(
                height: 42,
                padding:
                    const EdgeInsets.symmetric(horizontal: 14),
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.14),
                  borderRadius: BorderRadius.circular(21),
                  border: Border.all(
                      color: Colors.white.withValues(alpha: 0.15)),
                ),
                child: Row(
                  children: [
                    const Icon(Icons.chat_bubble_outline,
                        color: Colors.white54, size: 16),
                    const SizedBox(width: 8),
                    const Expanded(
                      child: Text(
                        'Say something...',
                        style: TextStyle(
                            color: Colors.white54, fontSize: 13),
                      ),
                    ),
                    // Heart tap shortcut
                    GestureDetector(
                      onTap: _spawnHeart,
                      child: const Icon(Icons.favorite_border,
                          color: Colors.white54, size: 18),
                    ),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(width: 10),
          // Gift
          _BarButton(
            icon: Icons.card_giftcard,
            color: const Color(0xFFFF2D55),
            onTap: () {
              GiftSheet.show(
                context: context,
                streamId: widget.streamId,
                onSendGift: _sendGift,
                gifts: _provider.giftCatalog.isNotEmpty
                    ? _provider.giftCatalog
                        .map((g) => GiftModel(
                              id: g.id,
                              name: g.name,
                              coinCost: g.coinPrice,
                              animationKey: g.animationUrl,
                              previewImageUrl: g.iconUrl,
                              category: g.category,
                            ))
                        .toList()
                    : null,
              );
            },
          ),
          const SizedBox(width: 8),
          // Coins top-up
          _BarButton(
            icon: Icons.monetization_on_outlined,
            color: Colors.white.withValues(alpha: 0.18),
            onTap: _showTopUpSheet,
          ),
          const SizedBox(width: 8),
          // Share
          _BarButton(
            icon: Icons.ios_share_outlined,
            color: Colors.white.withValues(alpha: 0.18),
            onTap: () {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text('Share link copied'),
                  behavior: SnackBarBehavior.floating,
                ),
              );
            },
          ),
        ],
      ),
    );
  }

  void _showChatInput() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => _ChatInputSheet(
        onSend: (text) async {
          await _provider.sendMessage(text);
          if (ctx.mounted) Navigator.pop(ctx);
        },
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Ended overlay
  // ---------------------------------------------------------------------------

  Widget _buildEndedOverlay() {
    return Container(
      color: Colors.black.withValues(alpha: 0.88),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.live_tv,
                color: Color(0xFF333333), size: 72),
            const SizedBox(height: 20),
            const Text(
              'Stream has ended',
              style: TextStyle(
                color: Colors.white,
                fontSize: 22,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            const Text(
              'Thanks for watching!',
              style: TextStyle(color: Color(0xFF888888), fontSize: 14),
            ),
            const SizedBox(height: 32),
            ElevatedButton(
              onPressed: () => Navigator.pop(context),
              style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFFFF2D55),
                padding: const EdgeInsets.symmetric(
                    horizontal: 36, vertical: 14),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
              ),
              child: const Text(
                'Leave stream',
                style: TextStyle(
                    color: Colors.white, fontWeight: FontWeight.bold),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Double-tap hint
// ---------------------------------------------------------------------------

class _DoubleTapHint extends StatefulWidget {
  const _DoubleTapHint();

  @override
  State<_DoubleTapHint> createState() => _DoubleTapHintState();
}

class _DoubleTapHintState extends State<_DoubleTapHint>
    with SingleTickerProviderStateMixin {
  late AnimationController _ctrl;
  bool _visible = true;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 800),
    );
    _ctrl.forward();
    Future.delayed(const Duration(seconds: 3), () {
      if (mounted) setState(() => _visible = false);
    });
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_visible) return const SizedBox.shrink();
    return FadeTransition(
      opacity: _ctrl,
      child: Container(
        padding:
            const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.5),
          borderRadius: BorderRadius.circular(12),
        ),
        child: const Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.favorite, color: Color(0xFFFF2D55), size: 14),
            SizedBox(width: 5),
            Text('Double-tap to react',
                style: TextStyle(color: Colors.white70, fontSize: 11)),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Shared small widgets
// ---------------------------------------------------------------------------

class _CircleButton extends StatelessWidget {
  const _CircleButton({required this.icon, required this.onTap});
  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 36,
        height: 36,
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.5),
          shape: BoxShape.circle,
        ),
        child: Icon(icon, color: Colors.white, size: 16),
      ),
    );
  }
}

class _BarButton extends StatelessWidget {
  const _BarButton({
    required this.icon,
    required this.color,
    required this.onTap,
  });
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 42,
        height: 42,
        decoration: BoxDecoration(color: color, shape: BoxShape.circle),
        child: Icon(icon, color: Colors.white, size: 20),
      ),
    );
  }
}

class _ChatInputSheet extends StatefulWidget {
  const _ChatInputSheet({required this.onSend});
  final Future<void> Function(String) onSend;

  @override
  State<_ChatInputSheet> createState() => _ChatInputSheetState();
}

class _ChatInputSheetState extends State<_ChatInputSheet> {
  final _ctrl = TextEditingController();
  bool _sending = false;

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        16,
        16,
        16,
        MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _ctrl,
              autofocus: true,
              maxLength: 200,
              style: const TextStyle(color: Colors.white),
              decoration: InputDecoration(
                hintText: 'Say something...',
                hintStyle:
                    const TextStyle(color: Color(0xFF555555)),
                filled: true,
                fillColor: const Color(0xFF2A2A2A),
                counterText: '',
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                  borderSide: BorderSide.none,
                ),
                contentPadding: const EdgeInsets.symmetric(
                    horizontal: 14, vertical: 10),
              ),
            ),
          ),
          const SizedBox(width: 10),
          GestureDetector(
            onTap: _sending
                ? null
                : () async {
                    final text = _ctrl.text.trim();
                    if (text.isEmpty) return;
                    setState(() => _sending = true);
                    await widget.onSend(text);
                  },
            child: Container(
              width: 44,
              height: 44,
              decoration: const BoxDecoration(
                color: Color(0xFFFF2D55),
                shape: BoxShape.circle,
              ),
              child: _sending
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white),
                    )
                  : const Icon(Icons.send,
                      color: Colors.white, size: 18),
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Top-up sheet
// ---------------------------------------------------------------------------

class _TopUpSheet extends StatefulWidget {
  const _TopUpSheet();

  @override
  State<_TopUpSheet> createState() => _TopUpSheetState();
}

class _TopUpSheetState extends State<_TopUpSheet> {
  int _selectedIndex = 0;

  static const _packages = [
    _CoinPackage(coins: 7, price: 1, label: '\$1'),
    _CoinPackage(coins: 42, price: 6, label: '\$6'),
    _CoinPackage(coins: 210, price: 30, label: '\$30'),
    _CoinPackage(coins: 686, price: 98, label: '\$98'),
    _CoinPackage(coins: 2086, price: 298, label: '\$298'),
    _CoinPackage(coins: 3626, price: 518, label: '\$518'),
  ];

  @override
  Widget build(BuildContext context) {
    final selected = _packages[_selectedIndex];
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Text(
                  'Top Up',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                    color: Colors.black,
                  ),
                ),
                const Spacer(),
                IconButton(
                  icon: const Icon(Icons.help_outline,
                      color: Colors.grey, size: 20),
                  onPressed: () {},
                ),
              ],
            ),
            const Text(
              'Balance: 0 Coins',
              style: TextStyle(color: Colors.grey, fontSize: 13),
            ),
            const SizedBox(height: 16),
            GridView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              gridDelegate:
                  const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
                crossAxisSpacing: 10,
                mainAxisSpacing: 10,
                childAspectRatio: 1.3,
              ),
              itemCount: _packages.length,
              itemBuilder: (context, index) {
                final pkg = _packages[index];
                final isSel = _selectedIndex == index;
                return GestureDetector(
                  onTap: () => setState(() => _selectedIndex = index),
                  child: Container(
                    decoration: BoxDecoration(
                      border: Border.all(
                        color: isSel
                            ? const Color(0xFFFFAA00)
                            : Colors.grey.shade300,
                        width: isSel ? 2 : 1,
                      ),
                      borderRadius: BorderRadius.circular(8),
                      color:
                          isSel ? const Color(0xFFFFFAF0) : Colors.white,
                    ),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Text(
                          '${pkg.coins} Coins',
                          style: TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 15,
                            color: isSel
                                ? const Color(0xFFFFAA00)
                                : Colors.black,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          pkg.label,
                          style: const TextStyle(
                              color: Colors.grey, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              height: 50,
              child: ElevatedButton(
                onPressed: () {
                  Navigator.pop(context);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(
                        'Top Up \$${selected.price} successful',
                        style: const TextStyle(color: Colors.white),
                      ),
                      backgroundColor: const Color(0xFFFF2D55),
                      behavior: SnackBarBehavior.floating,
                    ),
                  );
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFF2D55),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                child: Text(
                  'Top Up \$${selected.price}',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
            ),
            const SizedBox(height: 8),
            const Center(
              child: Text(
                'By topping up you agree to the Coins Recharge Agreement',
                style: TextStyle(color: Colors.grey, fontSize: 11),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CoinPackage {
  final int coins, price;
  final String label;
  const _CoinPackage({
    required this.coins,
    required this.price,
    required this.label,
  });
}
