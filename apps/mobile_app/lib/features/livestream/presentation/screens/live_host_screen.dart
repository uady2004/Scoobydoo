import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';

import '../../../../core/network/api_client.dart';
import '../../providers/livestream_provider.dart';
import '../../repositories/livestream_repository.dart';
import '../../services/livestream_websocket_service.dart';
import '../../widgets/gift_animation_overlay.dart';
import '../../widgets/live_chat_overlay.dart';

class LiveHostScreen extends StatefulWidget {
  const LiveHostScreen({
    super.key,
    required this.title,
    this.category = '',
    this.isPrivate = false,
  });

  final String title;
  final String category;
  final bool isPrivate;

  @override
  State<LiveHostScreen> createState() => _LiveHostScreenState();
}

class _LiveHostScreenState extends State<LiveHostScreen> {
  late final LivestreamHostProvider _host;
  late final LivestreamViewerProvider _viewer;

  bool _endingStream = false;

  // Live duration timer
  int _durationSeconds = 0;
  Timer? _durationTimer;

  // Mock viewer simulation
  int _mockViewerCount = 0;
  Timer? _viewerSimTimer;

  // Gift leaderboard
  final List<_GifterEntry> _topGifters = [];

  @override
  void initState() {
    super.initState();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);

    _host = LivestreamHostProvider(
      repo: LivestreamRepository(dio: ApiClient.instance.dio),
      wsService: LivestreamWebSocketService(
        wsBaseUrl: 'wss://api.tiktokclone.com/ws',
        authToken: '',
      ),
    );

    _viewer = LivestreamViewerProvider(
      repo: LivestreamRepository(dio: ApiClient.instance.dio),
      wsService: LivestreamWebSocketService(
        wsBaseUrl: 'wss://api.tiktokclone.com/ws',
        authToken: '',
      ),
    );

    _host.setTitle(widget.title);
    _host.addListener(_onHostUpdate);
    _viewer.addListener(_onViewerUpdate);
    _startStream();
    _startMockSimulation();
  }

  Future<void> _startStream() async {
    await _host.startStream();
    final streamId = _host.stream?.id;
    if (streamId != null) {
      await _viewer.joinStream(streamId);
    }
  }

  void _startMockSimulation() {
    // Ramp up viewers gradually
    _durationTimer =
        Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() => _durationSeconds++);
    });

    _viewerSimTimer =
        Timer.periodic(const Duration(seconds: 4), (_) {
      if (!mounted) return;
      setState(() {
        if (_mockViewerCount < 500) {
          _mockViewerCount += 7 + (_mockViewerCount ~/ 20);
        } else {
          _mockViewerCount +=
              (_mockViewerCount * 0.02).round().clamp(1, 80);
        }
      });
    });

    // Seed initial viewer count
    Future.delayed(const Duration(milliseconds: 800), () {
      if (mounted) setState(() => _mockViewerCount = 12);
    });
  }

  void _onHostUpdate() {
    if (mounted) setState(() {});
  }

  void _onViewerUpdate() {
    if (!mounted) return;
    for (final gift in _viewer.giftQueue) {
      _updateLeaderboard(gift.senderName, gift.totalCoins);
    }
    setState(() {});
  }

  void _updateLeaderboard(String sender, int coins) {
    final idx = _topGifters.indexWhere((e) => e.name == sender);
    if (idx >= 0) {
      _topGifters[idx] = _GifterEntry(
        name: _topGifters[idx].name,
        totalCoins: _topGifters[idx].totalCoins + coins,
      );
    } else {
      _topGifters.add(_GifterEntry(name: sender, totalCoins: coins));
    }
    _topGifters.sort((a, b) => b.totalCoins.compareTo(a.totalCoins));
    if (_topGifters.length > 3) {
      _topGifters.removeRange(3, _topGifters.length);
    }
  }

  Future<void> _endStream() async {
    final confirmed = await _showEndConfirmDialog();
    if (!confirmed || !mounted) return;
    setState(() => _endingStream = true);
    await _host.endStream();
    if (mounted) Navigator.pop(context);
  }

  Future<bool> _showEndConfirmDialog() async {
    return await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            backgroundColor: const Color(0xFF1A1A1A),
            shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(16)),
            title: const Text(
              'End stream?',
              style: TextStyle(color: Colors.white),
            ),
            content: Text(
              '${_fmtDuration(_durationSeconds)} streamed • $_mockViewerCount viewers',
              style: const TextStyle(color: Color(0xFF888888)),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('Continue',
                    style: TextStyle(color: Color(0xFF888888))),
              ),
              TextButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('End stream',
                    style: TextStyle(color: Color(0xFFFF2D55))),
              ),
            ],
          ),
        ) ??
        false;
  }

  String _fmtDuration(int totalSeconds) {
    final m = totalSeconds ~/ 60;
    final s = totalSeconds % 60;
    if (m >= 60) {
      final h = m ~/ 60;
      final rm = m % 60;
      return '${h}h ${rm}m';
    }
    return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
  }

  int get _displayViewerCount =>
      _viewer.viewerCount > 0 ? _viewer.viewerCount : _mockViewerCount;

  @override
  void dispose() {
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    _durationTimer?.cancel();
    _viewerSimTimer?.cancel();
    _host.removeListener(_onHostUpdate);
    _viewer.removeListener(_onViewerUpdate);
    _host.dispose();
    _viewer.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        fit: StackFit.expand,
        children: [
          // Camera preview (simulated)
          _buildCameraPreview(),

          // Top bar
          _buildTopBar(),

          // Right viewer avatars
          _buildViewerAvatarColumn(),

          // Gift leaderboard
          if (_topGifters.isNotEmpty) _buildLeaderboard(),

          // Chat overlay
          Positioned(
            bottom: 88,
            left: 0,
            width: MediaQuery.of(context).size.width * 0.72,
            height: MediaQuery.of(context).size.height * 0.30,
            child: LiveChatOverlay(
              messages: _viewer.messages,
              pinnedMessage: _viewer.pinnedMessage,
              onSendMessage: _viewer.sendMessage,
              allowComments: true,
              isHost: true,
              onDeleteMessage: _host.stream != null
                  ? (id) => _host.deleteMessage(id)
                  : null,
            ),
          ),

          // Gift animation layer
          if (_viewer.giftQueue.isNotEmpty)
            GiftAnimationOverlay(
              gift: _viewer.giftQueue.first,
              onDismiss: _viewer.dismissGiftAnimation,
            ),

          // Bottom action bar
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: _buildBottomBar(),
          ),

          // Ending overlay
          if (_endingStream)
            Container(
              color: Colors.black.withValues(alpha: 0.75),
              child: const Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    CircularProgressIndicator(color: Color(0xFFFF2D55)),
                    SizedBox(height: 16),
                    Text('Ending stream...',
                        style: TextStyle(
                            color: Colors.white, fontSize: 16)),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Camera preview
  // ---------------------------------------------------------------------------

  Widget _buildCameraPreview() {
    return Container(
      decoration: const BoxDecoration(
        gradient: RadialGradient(
          center: Alignment(0, -0.2),
          radius: 1.3,
          colors: [Color(0xFF1A1A2E), Color(0xFF0A0A0A)],
        ),
      ),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.camera_front,
                color: Color(0xFF1E1E2E), size: 80),
            const SizedBox(height: 8),
            // Simulated "you're live" glow text
            Container(
              padding: const EdgeInsets.symmetric(
                  horizontal: 20, vertical: 8),
              decoration: BoxDecoration(
                color: const Color(0xFFFF2D55).withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                    color: const Color(0xFFFF2D55)
                        .withValues(alpha: 0.3)),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: 8,
                    height: 8,
                    decoration: const BoxDecoration(
                      color: Color(0xFFFF2D55),
                      shape: BoxShape.circle,
                    ),
                  ),
                  const SizedBox(width: 8),
                  const Text('YOU\'RE LIVE',
                      style: TextStyle(
                        color: Color(0xFFFF2D55),
                        fontSize: 13,
                        fontWeight: FontWeight.bold,
                        letterSpacing: 1.5,
                      )),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Top bar
  // ---------------------------------------------------------------------------

  Widget _buildTopBar() {
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
              Colors.black.withValues(alpha: 0.85),
              Colors.transparent,
            ],
          ),
        ),
        child: Row(
          children: [
            // Live badge + title
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: const Color(0xFFFF2D55),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: const Text(
                          'LIVE',
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 11,
                            fontWeight: FontWeight.bold,
                            letterSpacing: 1,
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        _fmtDuration(_durationSeconds),
                        style: const TextStyle(
                          color: Colors.white70,
                          fontSize: 12,
                          fontFamily: 'monospace',
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    widget.title,
                    style: const TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.bold,
                      fontSize: 13,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  if (widget.category.isNotEmpty)
                    Text(
                      widget.category,
                      style: const TextStyle(
                          color: Colors.white54, fontSize: 11),
                    ),
                ],
              ),
            ),
            // Viewer count
            _ViewerCountChip(count: _displayViewerCount),
            const SizedBox(width: 10),
            // End stream
            GestureDetector(
              onTap: _endingStream ? null : _endStream,
              child: Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: 14, vertical: 8),
                decoration: BoxDecoration(
                  color: const Color(0xFFFF2D55),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: const Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.close, color: Colors.white, size: 14),
                    SizedBox(width: 4),
                    Text(
                      'End',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 13,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Viewer avatar column
  // ---------------------------------------------------------------------------

  Widget _buildViewerAvatarColumn() {
    final avatarCount = _displayViewerCount.clamp(0, 5);
    return Positioned(
      top: MediaQuery.of(context).padding.top + 72,
      right: 12,
      child: Column(
        children: List.generate(avatarCount, (i) {
          return Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: _ViewerAvatar(index: i),
          );
        }),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Gift leaderboard
  // ---------------------------------------------------------------------------

  Widget _buildLeaderboard() {
    return Positioned(
      top: MediaQuery.of(context).padding.top + 68,
      left: 12,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Top gifters',
            style: TextStyle(
              color: Colors.white60,
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 6),
          ..._topGifters.asMap().entries.map((e) => _LeaderboardRow(
                rank: e.key + 1,
                name: e.value.name,
                coins: e.value.totalCoins,
              )),
        ],
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Bottom action bar
  // ---------------------------------------------------------------------------

  Widget _buildBottomBar() {
    return Container(
      padding: EdgeInsets.fromLTRB(
        16,
        8,
        16,
        MediaQuery.of(context).padding.bottom + 10,
      ),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.bottomCenter,
          end: Alignment.topCenter,
          colors: [
            Colors.black.withValues(alpha: 0.9),
            Colors.transparent,
          ],
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceAround,
        children: [
          _HostActionBtn(
            icon: Icons.chat_bubble_outline,
            label: 'Chat',
            onTap: _showChatInput,
          ),
          _HostActionBtn(
            icon: Icons.auto_awesome_outlined,
            label: 'Effects',
            onTap: () => _snack('Effects coming soon'),
          ),
          _HostActionBtn(
            icon: Icons.card_giftcard_outlined,
            label: 'Gifts',
            onTap: () => _snack('Gift stats coming soon'),
            color: const Color(0xFFFF2D55),
          ),
          _HostActionBtn(
            icon: Icons.flip_camera_ios_outlined,
            label: 'Flip',
            onTap: () {},
          ),
          _HostActionBtn(
            icon: Icons.share_outlined,
            label: 'Share',
            onTap: () => _snack('Share link copied'),
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
          await _viewer.sendMessage(text);
          if (ctx.mounted) Navigator.pop(ctx);
        },
      ),
    );
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(msg, style: const TextStyle(color: Colors.white)),
        backgroundColor: const Color(0xFF2A2A2A),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Viewer count chip
// ---------------------------------------------------------------------------

class _ViewerCountChip extends StatelessWidget {
  const _ViewerCountChip({required this.count});
  final int count;

  static String _fmt(int n) {
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
    return '$n';
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: Colors.white.withValues(alpha: 0.2)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.remove_red_eye_outlined,
              color: Colors.white70, size: 14),
          const SizedBox(width: 5),
          Text(
            _fmt(count),
            style: const TextStyle(
              color: Colors.white,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Host action button
// ---------------------------------------------------------------------------

class _HostActionBtn extends StatelessWidget {
  const _HostActionBtn({
    required this.icon,
    required this.label,
    required this.onTap,
    this.color,
  });

  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final c = color ?? Colors.white;
    return GestureDetector(
      onTap: onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: color != null
                  ? color!.withValues(alpha: 0.15)
                  : Colors.white.withValues(alpha: 0.1),
              shape: BoxShape.circle,
              border: Border.all(
                color: color != null
                    ? color!.withValues(alpha: 0.4)
                    : Colors.white.withValues(alpha: 0.2),
              ),
            ),
            child: Icon(icon, color: c, size: 22),
          ),
          const SizedBox(height: 5),
          Text(
            label,
            style: const TextStyle(
              color: Colors.white70,
              fontSize: 11,
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Leaderboard
// ---------------------------------------------------------------------------

class _GifterEntry {
  const _GifterEntry({required this.name, required this.totalCoins});
  final String name;
  final int totalCoins;
}

class _LeaderboardRow extends StatelessWidget {
  const _LeaderboardRow({
    required this.rank,
    required this.name,
    required this.coins,
  });

  final int rank;
  final String name;
  final int coins;

  static const _crownColors = [
    Color(0xFFFFD700),
    Color(0xFFB0C4DE),
    Color(0xFFCD853F),
  ];

  @override
  Widget build(BuildContext context) {
    final crownColor =
        rank <= 3 ? _crownColors[rank - 1] : Colors.white38;
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.workspace_premium, color: crownColor, size: 16),
          const SizedBox(width: 4),
          Text(
            name,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 12,
              fontWeight: FontWeight.w600,
              shadows: [Shadow(blurRadius: 2, color: Colors.black)],
            ),
          ),
          const SizedBox(width: 6),
          const Icon(Icons.monetization_on,
              color: Color(0xFFFFD700), size: 11),
          const SizedBox(width: 2),
          Text(
            NumberFormat.compact().format(coins),
            style: const TextStyle(
              color: Color(0xFFFFD700),
              fontSize: 11,
              shadows: [Shadow(blurRadius: 2, color: Colors.black)],
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Viewer avatar
// ---------------------------------------------------------------------------

class _ViewerAvatar extends StatelessWidget {
  const _ViewerAvatar({required this.index});
  final int index;

  static const _colors = [
    Color(0xFFFF2D55),
    Color(0xFF4CAF50),
    Color(0xFF2196F3),
    Color(0xFFFF9800),
    Color(0xFF9C27B0),
  ];

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 34,
      height: 34,
      decoration: BoxDecoration(
        color: _colors[index % _colors.length],
        shape: BoxShape.circle,
        border: Border.all(color: Colors.black, width: 1.5),
      ),
      child: const Icon(Icons.person, color: Colors.white, size: 18),
    );
  }
}

// ---------------------------------------------------------------------------
// Chat input sheet
// ---------------------------------------------------------------------------

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
