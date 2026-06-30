import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../providers/livestream_provider.dart';

/// Bottom sheet the host sees before going live — collects title, description
/// and stream settings before calling [LivestreamHostProvider.startStream].
class GoLiveSetupSheet extends StatelessWidget {
  const GoLiveSetupSheet({super.key});

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<LivestreamHostProvider>();

    return Container(
      padding: EdgeInsets.only(
        left: 24,
        right: 24,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 24,
      ),
      decoration: const BoxDecoration(
        color: Color(0xFF1A1A1A),
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _handle(),
          const Text(
            'Go Live',
            style: TextStyle(
              color: Colors.white,
              fontSize: 20,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 20),
          _titleField(provider),
          const SizedBox(height: 12),
          _descField(provider),
          const SizedBox(height: 16),
          _commentsToggle(provider),
          const SizedBox(height: 24),
          if (provider.error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(
                provider.error!,
                style: const TextStyle(color: Colors.redAccent, fontSize: 13),
              ),
            ),
          _goLiveButton(context, provider),
        ],
      ),
    );
  }

  Widget _handle() {
    return Center(
      child: Container(
        width: 36,
        height: 4,
        margin: const EdgeInsets.only(bottom: 14),
        decoration: BoxDecoration(
          color: Colors.white.withValues(alpha: 0.3),
          borderRadius: BorderRadius.circular(2),
        ),
      ),
    );
  }

  Widget _titleField(LivestreamHostProvider provider) {
    return TextField(
      maxLength: 100,
      style: const TextStyle(color: Colors.white),
      decoration: InputDecoration(
        labelText: 'Stream Title *',
        labelStyle: const TextStyle(color: Colors.white54),
        counterStyle: const TextStyle(color: Colors.white38),
        filled: true,
        fillColor: Colors.white.withValues(alpha: 0.08),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
      ),
      onChanged: provider.setTitle,
    );
  }

  Widget _descField(LivestreamHostProvider provider) {
    return TextField(
      maxLength: 300,
      maxLines: 2,
      style: const TextStyle(color: Colors.white),
      decoration: InputDecoration(
        labelText: 'Description (optional)',
        labelStyle: const TextStyle(color: Colors.white54),
        counterStyle: const TextStyle(color: Colors.white38),
        filled: true,
        fillColor: Colors.white.withValues(alpha: 0.08),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
      ),
      onChanged: provider.setDescription,
    );
  }

  Widget _commentsToggle(LivestreamHostProvider provider) {
    return Row(
      children: [
        const Icon(Icons.chat_bubble_outline, color: Colors.white70, size: 18),
        const SizedBox(width: 10),
        const Expanded(
          child: Text(
            'Allow Comments',
            style: TextStyle(color: Colors.white, fontSize: 14),
          ),
        ),
        Switch(
          value: provider.allowComments,
          onChanged: (_) => provider.toggleAllowComments(),
          activeThumbColor: const Color(0xFFFF2D55),
        ),
      ],
    );
  }

  Widget _goLiveButton(BuildContext context, LivestreamHostProvider provider) {
    return SizedBox(
      width: double.infinity,
      height: 52,
      child: ElevatedButton(
        onPressed: provider.loading ? null : () => _goLive(context, provider),
        style: ElevatedButton.styleFrom(
          backgroundColor: const Color(0xFFFF2D55),
          disabledBackgroundColor: Colors.grey.shade800,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(26),
          ),
        ),
        child: provider.loading
            ? const SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: Colors.white,
                ),
              )
            : const Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(Icons.videocam, color: Colors.white),
                  SizedBox(width: 8),
                  Text(
                    'Go Live',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
              ),
      ),
    );
  }

  Future<void> _goLive(
    BuildContext context,
    LivestreamHostProvider provider,
  ) async {
    final rtmpKey = await provider.startStream();
    if (rtmpKey != null && context.mounted) {
      // Close setup sheet and navigate to the host broadcast screen.
      Navigator.pop(context, rtmpKey);
    }
  }
}
