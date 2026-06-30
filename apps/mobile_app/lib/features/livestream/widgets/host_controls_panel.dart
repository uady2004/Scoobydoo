import 'package:flutter/material.dart';

/// Row of host-only action buttons rendered at the bottom of the
/// livestream screen when the current user is the stream host.
class HostControlsPanel extends StatelessWidget {
  const HostControlsPanel({
    super.key,
    required this.onEndStream,
    required this.onFlipCamera,
    required this.onToggleMic,
    required this.onInvitePKBattle,
    required this.onCreatePoll,
    required this.onShareLink,
    this.isMicMuted = false,
    this.hasPKBattleActive = false,
  });

  final VoidCallback onEndStream;
  final VoidCallback onFlipCamera;
  final VoidCallback onToggleMic;
  final VoidCallback onInvitePKBattle;
  final VoidCallback onCreatePoll;
  final VoidCallback onShareLink;
  final bool isMicMuted;
  final bool hasPKBattleActive;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceEvenly,
      children: [
        _ControlButton(
          icon: isMicMuted ? Icons.mic_off : Icons.mic,
          label: isMicMuted ? 'Unmute' : 'Mute',
          color: isMicMuted ? Colors.red : Colors.white,
          onTap: onToggleMic,
        ),
        _ControlButton(
          icon: Icons.flip_camera_ios,
          label: 'Flip',
          onTap: onFlipCamera,
        ),
        _ControlButton(
          icon: Icons.sports_kabaddi,
          label: 'PK Battle',
          color: hasPKBattleActive ? Colors.amber : Colors.white,
          onTap: onInvitePKBattle,
        ),
        _ControlButton(
          icon: Icons.poll,
          label: 'Poll',
          onTap: onCreatePoll,
        ),
        _ControlButton(
          icon: Icons.share,
          label: 'Share',
          onTap: onShareLink,
        ),
        _ControlButton(
          icon: Icons.close,
          label: 'End',
          color: const Color(0xFFFF2D55),
          onTap: () => _confirmEnd(context),
        ),
      ],
    );
  }

  void _confirmEnd(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('End Stream?',
            style: TextStyle(color: Colors.white)),
        content: const Text(
          'Your stream will end and viewers will be disconnected.',
          style: TextStyle(color: Colors.white70),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFFFF2D55)),
            onPressed: () {
              Navigator.pop(ctx);
              onEndStream();
            },
            child: const Text('End Stream',
                style: TextStyle(color: Colors.white)),
          ),
        ],
      ),
    );
  }
}

class _ControlButton extends StatelessWidget {
  const _ControlButton({
    required this.icon,
    required this.label,
    required this.onTap,
    this.color = Colors.white,
  });

  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: Colors.white.withValues(alpha: 0.12),
              shape: BoxShape.circle,
            ),
            child: Icon(icon, color: color, size: 22),
          ),
          const SizedBox(height: 4),
          Text(
            label,
            style: TextStyle(
              color: color.withValues(alpha: 0.85),
              fontSize: 10,
            ),
          ),
        ],
      ),
    );
  }
}
