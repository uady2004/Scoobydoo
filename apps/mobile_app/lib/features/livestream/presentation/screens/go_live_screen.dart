import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'live_host_screen.dart';

class GoLiveScreen extends StatefulWidget {
  const GoLiveScreen({super.key});

  @override
  State<GoLiveScreen> createState() => _GoLiveScreenState();
}

class _GoLiveScreenState extends State<GoLiveScreen>
    with SingleTickerProviderStateMixin {
  final _titleController = TextEditingController();
  String _selectedCategory = '';
  bool _isPrivate = false;
  bool _frontCamera = true;
  bool _flashOn = false;

  bool _countingDown = false;
  int _countdown = 3;
  Timer? _countdownTimer;

  static const List<Map<String, dynamic>> _categories = [
    {'label': 'Entertainment', 'icon': Icons.celebration_outlined},
    {'label': 'Gaming', 'icon': Icons.sports_esports_outlined},
    {'label': 'Music', 'icon': Icons.music_note_outlined},
    {'label': 'Sports', 'icon': Icons.sports_soccer_outlined},
    {'label': 'Cooking', 'icon': Icons.restaurant_outlined},
    {'label': 'Q&A', 'icon': Icons.question_answer_outlined},
    {'label': 'Education', 'icon': Icons.school_outlined},
    {'label': 'Travel', 'icon': Icons.flight_outlined},
  ];

  @override
  void initState() {
    super.initState();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
  }

  @override
  void dispose() {
    _titleController.dispose();
    _countdownTimer?.cancel();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    super.dispose();
  }

  void _startGoLive() {
    if (_titleController.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Add a title to describe your stream'),
          backgroundColor: Color(0xFFFF2D55),
          behavior: SnackBarBehavior.floating,
        ),
      );
      return;
    }

    setState(() {
      _countingDown = true;
      _countdown = 3;
    });

    _countdownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      if (_countdown <= 1) {
        timer.cancel();
        _navigateToHost();
      } else {
        setState(() => _countdown--);
      }
    });
  }

  void _navigateToHost() {
    if (!mounted) return;
    Navigator.pushReplacement(
      context,
      MaterialPageRoute<void>(
        builder: (_) => LiveHostScreen(
          title: _titleController.text.trim(),
          category: _selectedCategory,
          isPrivate: _isPrivate,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        fit: StackFit.expand,
        children: [
          // Simulated camera preview
          _CameraPreviewBg(frontCamera: _frontCamera),

          // Top controls
          Positioned(
            top: MediaQuery.of(context).padding.top + 8,
            left: 12,
            right: 12,
            child: Row(
              children: [
                _TopIconButton(
                  icon: Icons.close,
                  onTap: () => Navigator.pop(context),
                ),
                const Spacer(),
                // Beauty
                _TopIconButton(
                  icon: Icons.face_retouching_natural,
                  onTap: () {},
                ),
                const SizedBox(width: 10),
                // Flash toggle
                _TopIconButton(
                  icon: _flashOn ? Icons.flash_on : Icons.flash_off,
                  onTap: () => setState(() => _flashOn = !_flashOn),
                  active: _flashOn,
                ),
                const SizedBox(width: 10),
                // Flip camera
                _TopIconButton(
                  icon: Icons.flip_camera_ios,
                  onTap: () => setState(() => _frontCamera = !_frontCamera),
                ),
              ],
            ),
          ),

          // Live LIVE badge top center
          Positioned(
            top: MediaQuery.of(context).padding.top + 16,
            left: 0,
            right: 0,
            child: Center(
              child: Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: 16, vertical: 5),
                decoration: BoxDecoration(
                  color: const Color(0xFFFF2D55),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: const Text(
                  'LIVE',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.bold,
                    letterSpacing: 1.5,
                  ),
                ),
              ),
            ),
          ),

          // Bottom setup panel
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: _buildSetupPanel(),
          ),

          // Countdown overlay
          if (_countingDown) _buildCountdownOverlay(),
        ],
      ),
    );
  }

  Widget _buildSetupPanel() {
    return Container(
      padding: EdgeInsets.fromLTRB(
        20,
        24,
        20,
        MediaQuery.of(context).padding.bottom + 20,
      ),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.bottomCenter,
          end: Alignment.topCenter,
          colors: [
            Colors.black.withValues(alpha: 0.96),
            Colors.black.withValues(alpha: 0.75),
            Colors.transparent,
          ],
          stops: const [0.0, 0.65, 1.0],
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Stats row
          const Row(
            children: [
              _StatBadge(
                icon: Icons.people_outline,
                label: 'Avg 1.2K viewers',
              ),
              SizedBox(width: 10),
              _StatBadge(
                icon: Icons.monetization_on_outlined,
                label: 'Earn coins',
                color: Color(0xFFFFD700),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Title input
          TextField(
            controller: _titleController,
            maxLength: 80,
            style: const TextStyle(color: Colors.white, fontSize: 15),
            decoration: InputDecoration(
              hintText: 'Give your stream a title...',
              hintStyle:
                  const TextStyle(color: Color(0xFF555555), fontSize: 15),
              filled: true,
              fillColor: Colors.white.withValues(alpha: 0.1),
              counterText: '',
              prefixIcon: const Icon(Icons.live_tv,
                  color: Color(0xFFFF2D55), size: 20),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
                borderSide: BorderSide.none,
              ),
              contentPadding: const EdgeInsets.symmetric(
                  horizontal: 16, vertical: 14),
            ),
          ),
          const SizedBox(height: 16),

          // Category label
          const Text(
            'Category',
            style: TextStyle(
              color: Color(0xFF888888),
              fontSize: 12,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.5,
            ),
          ),
          const SizedBox(height: 10),

          // Category chips grid
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: _categories.map((cat) {
              final label = cat['label'] as String;
              final icon = cat['icon'] as IconData;
              final selected = _selectedCategory == label;
              return GestureDetector(
                onTap: () => setState(
                    () => _selectedCategory = selected ? '' : label),
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 200),
                  padding: const EdgeInsets.symmetric(
                      horizontal: 12, vertical: 7),
                  decoration: BoxDecoration(
                    color: selected
                        ? const Color(0xFFFF2D55)
                        : Colors.white.withValues(alpha: 0.08),
                    borderRadius: BorderRadius.circular(20),
                    border: Border.all(
                      color: selected
                          ? Colors.transparent
                          : Colors.white.withValues(alpha: 0.18),
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        icon,
                        size: 14,
                        color: selected
                            ? Colors.white
                            : const Color(0xFFBBBBBB),
                      ),
                      const SizedBox(width: 5),
                      Text(
                        label,
                        style: TextStyle(
                          color: selected
                              ? Colors.white
                              : const Color(0xFFCCCCCC),
                          fontSize: 13,
                          fontWeight: selected
                              ? FontWeight.w600
                              : FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            }).toList(),
          ),
          const SizedBox(height: 16),

          // Settings row
          Row(
            children: [
              // Privacy toggle
              Expanded(
                child: Container(
                  padding: const EdgeInsets.symmetric(
                      horizontal: 14, vertical: 10),
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: 0.07),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.lock_outline,
                          color: Color(0xFF888888), size: 18),
                      const SizedBox(width: 8),
                      const Text(
                        'Private',
                        style: TextStyle(
                            color: Colors.white, fontSize: 13),
                      ),
                      const Spacer(),
                      Switch(
                        value: _isPrivate,
                        onChanged: (v) =>
                            setState(() => _isPrivate = v),
                        activeThumbColor: Colors.white,
                        activeTrackColor: const Color(0xFFFF2D55),
                        inactiveThumbColor: const Color(0xFF555555),
                        inactiveTrackColor: const Color(0xFF2A2A2A),
                        materialTapTargetSize:
                            MaterialTapTargetSize.shrinkWrap,
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 10),
              // Invite friends
              GestureDetector(
                onTap: () => ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(
                    content: Text('Invite friends coming soon'),
                    behavior: SnackBarBehavior.floating,
                  ),
                ),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                      horizontal: 14, vertical: 10),
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: 0.07),
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(
                        color: Colors.white.withValues(alpha: 0.15)),
                  ),
                  child: const Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.person_add_outlined,
                          color: Color(0xFF888888), size: 18),
                      SizedBox(width: 8),
                      Text(
                        'Invite',
                        style:
                            TextStyle(color: Colors.white, fontSize: 13),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 20),

          // Go Live button
          SizedBox(
            width: double.infinity,
            child: GestureDetector(
              onTap: _countingDown ? null : _startGoLive,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),
                padding: const EdgeInsets.symmetric(vertical: 16),
                decoration: BoxDecoration(
                  gradient: _countingDown
                      ? null
                      : const LinearGradient(
                          colors: [
                            Color(0xFFFF2D55),
                            Color(0xFFFF6B8A),
                          ],
                        ),
                  color: _countingDown
                      ? const Color(0xFF2A2A2A)
                      : null,
                  borderRadius: BorderRadius.circular(14),
                  boxShadow: _countingDown
                      ? null
                      : [
                          BoxShadow(
                            color: const Color(0xFFFF2D55)
                                .withValues(alpha: 0.45),
                            blurRadius: 20,
                            offset: const Offset(0, 6),
                          ),
                        ],
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(Icons.live_tv,
                        color: Colors.white, size: 22),
                    const SizedBox(width: 10),
                    Text(
                      _countingDown ? 'Starting in $_countdown...' : 'Go Live',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                        letterSpacing: 0.5,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCountdownOverlay() {
    return Container(
      color: Colors.black.withValues(alpha: 0.55),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _CountdownNumber(value: _countdown),
            const SizedBox(height: 16),
            const Text(
              'Going live...',
              style: TextStyle(
                color: Colors.white70,
                fontSize: 16,
                letterSpacing: 1,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Stat badge widget
// ---------------------------------------------------------------------------

class _StatBadge extends StatelessWidget {
  const _StatBadge({required this.icon, required this.label, this.color});
  final IconData icon;
  final String label;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final c = color ?? Colors.white70;
    return Container(
      padding:
          const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(20),
        border:
            Border.all(color: Colors.white.withValues(alpha: 0.12)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, color: c, size: 14),
          const SizedBox(width: 5),
          Text(label,
              style: TextStyle(color: c, fontSize: 12)),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Countdown number with scale animation
// ---------------------------------------------------------------------------

class _CountdownNumber extends StatefulWidget {
  const _CountdownNumber({required this.value});
  final int value;

  @override
  State<_CountdownNumber> createState() => _CountdownNumberState();
}

class _CountdownNumberState extends State<_CountdownNumber>
    with SingleTickerProviderStateMixin {
  late AnimationController _ctrl;
  late Animation<double> _scale;
  late Animation<double> _opacity;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 700),
    );
    _scale = CurvedAnimation(parent: _ctrl, curve: Curves.elasticOut);
    _opacity = CurvedAnimation(parent: _ctrl, curve: Curves.easeIn);
    _ctrl.forward();
  }

  @override
  void didUpdateWidget(_CountdownNumber old) {
    super.didUpdateWidget(old);
    if (old.value != widget.value) {
      _ctrl.forward(from: 0);
    }
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: _opacity,
      child: ScaleTransition(
        scale: _scale,
        child: Text(
          '${widget.value}',
          style: const TextStyle(
            color: Colors.white,
            fontSize: 130,
            fontWeight: FontWeight.bold,
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Simulated camera preview background
// ---------------------------------------------------------------------------

class _CameraPreviewBg extends StatelessWidget {
  const _CameraPreviewBg({required this.frontCamera});
  final bool frontCamera;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        gradient: RadialGradient(
          center: Alignment(0, frontCamera ? -0.2 : 0.1),
          radius: 1.2,
          colors: const [Color(0xFF1A1A2E), Color(0xFF080808)],
        ),
      ),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              frontCamera ? Icons.camera_front : Icons.camera_rear,
              color: const Color(0xFF222222),
              size: 72,
            ),
            const SizedBox(height: 10),
            Text(
              frontCamera ? 'Front camera' : 'Rear camera',
              style: const TextStyle(
                color: Color(0xFF333333),
                fontSize: 13,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Top icon button
// ---------------------------------------------------------------------------

class _TopIconButton extends StatelessWidget {
  const _TopIconButton({
    required this.icon,
    required this.onTap,
    this.active = false,
  });

  final IconData icon;
  final VoidCallback onTap;
  final bool active;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          color: active
              ? const Color(0xFFFF2D55).withValues(alpha: 0.85)
              : Colors.black.withValues(alpha: 0.5),
          shape: BoxShape.circle,
          border: Border.all(color: Colors.white.withValues(alpha: 0.15)),
        ),
        child: Icon(icon, color: Colors.white, size: 20),
      ),
    );
  }
}
