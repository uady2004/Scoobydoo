import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../ecommerce/domain/entities/order_entity.dart';
import '../../../ecommerce/domain/usecases/place_order_usecase.dart';
import '../../../ecommerce/presentation/providers/ecommerce_provider.dart';

// ─────────────────────────────────────────────────────────────────────────────
// CheckoutScreen
// ─────────────────────────────────────────────────────────────────────────────

class CheckoutScreen extends ConsumerStatefulWidget {
  const CheckoutScreen({super.key});

  @override
  ConsumerState<CheckoutScreen> createState() => _CheckoutScreenState();
}

class _CheckoutScreenState extends ConsumerState<CheckoutScreen> {
  // Address state
  BuyerInfoEntity? _address;

  // Payment state
  String _paymentMethod = 'card'; // 'card' | 'coins'
  final String _lastFour = '4242';
  final double _coinBalance = 1250.0;

  bool _placing = false;

  // ── Form helpers ────────────────────────────────────────────────────────────

  void _showAddAddress() {
    showModalBottomSheet<BuyerInfoEntity?>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF111111),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => const _AddressForm(),
    ).then((addr) {
      if (addr != null) setState(() => _address = addr);
    });
  }

  // ── Place order ─────────────────────────────────────────────────────────────

  Future<void> _placeOrder() async {
    if (_address == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          backgroundColor: Color(0xFF1A1A1A),
          content: Text('Please add a delivery address.',
              style: TextStyle(color: Colors.white)),
        ),
      );
      return;
    }

    setState(() => _placing = true);

    // Show loading dialog
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (_) => const _PlacingOrderDialog(),
    );

    final useCase = ref.read(placeOrderUseCaseProvider);
    final result = await useCase(PlaceOrderParams(
      shippingAddress: _address!,
      paymentMethod: _paymentMethod,
    ));

    if (!mounted) return;
    Navigator.of(context).pop(); // close loading dialog

    result.fold(
      (failure) {
        setState(() => _placing = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            backgroundColor: const Color(0xFF1A1A1A),
            content: Text(failure.message,
                style: const TextStyle(color: Colors.white)),
          ),
        );
      },
      (order) {
        setState(() => _placing = false);
        ref.read(cartProvider.notifier).clear();
        ref.read(ordersProvider.notifier).prependOrder(order);
        // Navigate to success screen (replaces checkout so back goes to shop)
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(
            builder: (_) => _OrderSuccessScreen(order: order),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final asyncCart = ref.watch(cartProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: Colors.white),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: const Text(
          'Checkout',
          style: TextStyle(
              color: Colors.white,
              fontSize: 18,
              fontWeight: FontWeight.w700),
        ),
      ),
      body: asyncCart.when(
        loading: () => const Center(
          child: CircularProgressIndicator(
              color: Color(0xFFFF2D55), strokeWidth: 2),
        ),
        error: (err, _) => Center(
          child: Text(err.toString(),
              style: const TextStyle(color: Colors.white70)),
        ),
        data: (cart) => Column(
          children: [
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  // ── Delivery address ─────────────────────────────────────
                  const _SectionHeader(label: 'Delivery Address'),
                  const SizedBox(height: 12),
                  if (_address == null)
                    _AddAddressButton(onTap: _showAddAddress)
                  else
                    _AddressCard(
                      address: _address!,
                      onEdit: _showAddAddress,
                    ),
                  const SizedBox(height: 24),

                  // ── Order items ──────────────────────────────────────────
                  const _SectionHeader(label: 'Order Items'),
                  const SizedBox(height: 12),
                  ...cart.items.map(
                    (item) => Padding(
                      padding: const EdgeInsets.only(bottom: 10),
                      child: _CompactItemRow(item: item),
                    ),
                  ),
                  const SizedBox(height: 24),

                  // ── Payment method ───────────────────────────────────────
                  const _SectionHeader(label: 'Payment Method'),
                  const SizedBox(height: 12),
                  _PaymentOption(
                    icon: Icons.credit_card,
                    title: 'Card ending in $_lastFour',
                    subtitle: 'Visa',
                    selected: _paymentMethod == 'card',
                    onTap: () =>
                        setState(() => _paymentMethod = 'card'),
                  ),
                  const SizedBox(height: 8),
                  _PaymentOption(
                    icon: Icons.monetization_on_outlined,
                    title: 'Coins',
                    subtitle:
                        '${_coinBalance.toStringAsFixed(0)} coins available',
                    selected: _paymentMethod == 'coins',
                    onTap: () =>
                        setState(() => _paymentMethod = 'coins'),
                  ),
                  const SizedBox(height: 8),
                  _AddPaymentButton(
                    onTap: () {
                      /* navigate to add card flow */
                    },
                  ),
                  const SizedBox(height: 24),

                  // ── Order summary ────────────────────────────────────────
                  const _SectionHeader(label: 'Order Summary'),
                  const SizedBox(height: 12),
                  _CheckoutSummaryCard(cart: cart),
                ],
              ),
            ),
            // ── Place order button ─────────────────────────────────────────
            _PlaceOrderBar(
              onPlaceOrder: _placing ? null : _placeOrder,
              total: asyncCart.valueOrNull?.total ?? 0,
            ),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Section header
// ─────────────────────────────────────────────────────────────────────────────

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) {
    return Text(
      label,
      style: const TextStyle(
          color: Colors.white, fontSize: 15, fontWeight: FontWeight.w700),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Address widgets
// ─────────────────────────────────────────────────────────────────────────────

class _AddAddressButton extends StatelessWidget {
  const _AddAddressButton({required this.onTap});
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(12),
          border:
              Border.all(color: const Color(0xFF333333), style: BorderStyle.solid),
        ),
        child: const Row(
          children: [
            Icon(Icons.add_location_alt_outlined,
                color: Color(0xFFFF2D55), size: 22),
            SizedBox(width: 12),
            Text('Add delivery address',
                style: TextStyle(
                    color: Color(0xFFFF2D55),
                    fontSize: 14,
                    fontWeight: FontWeight.w500)),
          ],
        ),
      ),
    );
  }
}

class _AddressCard extends StatelessWidget {
  const _AddressCard({required this.address, required this.onEdit});
  final BuyerInfoEntity address;
  final VoidCallback onEdit;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF111111),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFF333333)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.location_on,
              color: Color(0xFFFF2D55), size: 20),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(address.name,
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 14,
                        fontWeight: FontWeight.w600)),
                const SizedBox(height: 4),
                Text(address.fullAddress,
                    style: const TextStyle(
                        color: Color(0xFF888888), fontSize: 13, height: 1.4)),
                const SizedBox(height: 4),
                Text(address.phone,
                    style: const TextStyle(
                        color: Color(0xFF888888), fontSize: 13)),
              ],
            ),
          ),
          GestureDetector(
            onTap: onEdit,
            child: const Icon(Icons.edit_outlined,
                color: Color(0xFF888888), size: 18),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Compact item row
// ─────────────────────────────────────────────────────────────────────────────

class _CompactItemRow extends StatelessWidget {
  const _CompactItemRow({required this.item});
  final CartItemEntity item;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: SizedBox(
            width: 48,
            height: 48,
            child: item.product.thumbnailUrl.isNotEmpty
                ? Image.network(item.product.thumbnailUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) => Container(
                          color: const Color(0xFF2A2A2A),
                        ))
                : Container(color: const Color(0xFF2A2A2A)),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(item.product.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      color: Colors.white, fontSize: 13)),
              if (item.selectedVariant != null)
                Text(item.selectedVariant!.name,
                    style: const TextStyle(
                        color: Color(0xFF888888), fontSize: 12)),
            ],
          ),
        ),
        const SizedBox(width: 8),
        Column(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text('x${item.qty}',
                style: const TextStyle(
                    color: Color(0xFF888888), fontSize: 12)),
            Text('\$${item.subtotal.toStringAsFixed(2)}',
                style: const TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.w600)),
          ],
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Payment option
// ─────────────────────────────────────────────────────────────────────────────

class _PaymentOption extends StatelessWidget {
  const _PaymentOption({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: selected
                ? const Color(0xFFFF2D55)
                : const Color(0xFF2A2A2A),
            width: selected ? 1.5 : 1,
          ),
        ),
        child: Row(
          children: [
            Icon(icon,
                color: selected
                    ? const Color(0xFFFF2D55)
                    : const Color(0xFF888888),
                size: 22),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title,
                      style: TextStyle(
                          color: selected ? Colors.white : const Color(0xFFCCCCCC),
                          fontSize: 14,
                          fontWeight: FontWeight.w500)),
                  Text(subtitle,
                      style: const TextStyle(
                          color: Color(0xFF666666), fontSize: 12)),
                ],
              ),
            ),
            if (selected)
              const Icon(Icons.check_circle,
                  color: Color(0xFFFF2D55), size: 18),
          ],
        ),
      ),
    );
  }
}

class _AddPaymentButton extends StatelessWidget {
  const _AddPaymentButton({required this.onTap});
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: const Color(0xFF2A2A2A)),
        ),
        child: const Row(
          children: [
            Icon(Icons.add, color: Color(0xFF888888), size: 20),
            SizedBox(width: 12),
            Text('Add new card',
                style: TextStyle(
                    color: Color(0xFF888888),
                    fontSize: 14,
                    fontWeight: FontWeight.w400)),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Checkout summary card
// ─────────────────────────────────────────────────────────────────────────────

class _CheckoutSummaryCard extends StatelessWidget {
  const _CheckoutSummaryCard({required this.cart});
  final CartEntity cart;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF111111),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        children: [
          _Row(label: 'Subtotal', value: '\$${cart.subtotal.toStringAsFixed(2)}'),
          const SizedBox(height: 8),
          _Row(
            label: 'Shipping',
            value: cart.shippingFee == 0
                ? 'Free'
                : '\$${cart.shippingFee.toStringAsFixed(2)}',
          ),
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 10),
            child: Divider(color: Color(0xFF2A2A2A), thickness: 1, height: 1),
          ),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text('Total',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.w700)),
              Text('\$${cart.total.toStringAsFixed(2)}',
                  style: const TextStyle(
                      color: Color(0xFFFF2D55),
                      fontSize: 18,
                      fontWeight: FontWeight.w800)),
            ],
          ),
        ],
      ),
    );
  }
}

class _Row extends StatelessWidget {
  const _Row({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label,
            style: const TextStyle(
                color: Color(0xFF888888), fontSize: 14)),
        Text(value,
            style: const TextStyle(color: Colors.white, fontSize: 14)),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Place order bar
// ─────────────────────────────────────────────────────────────────────────────

class _PlaceOrderBar extends StatelessWidget {
  const _PlaceOrderBar({required this.onPlaceOrder, required this.total});
  final VoidCallback? onPlaceOrder;
  final double total;

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).padding.bottom;
    return Container(
      padding: EdgeInsets.fromLTRB(16, 12, 16, 12 + bottomPadding),
      decoration: const BoxDecoration(
        color: Color(0xFF0A0A0A),
        border: Border(top: BorderSide(color: Color(0xFF1A1A1A))),
      ),
      child: SizedBox(
        width: double.infinity,
        child: DecoratedBox(
          decoration: BoxDecoration(
            gradient: onPlaceOrder != null
                ? const LinearGradient(
                    colors: [Color(0xFFFF2D55), Color(0xFFFF6B35)],
                  )
                : const LinearGradient(
                    colors: [Color(0xFF333333), Color(0xFF333333)],
                  ),
            borderRadius: BorderRadius.circular(12),
          ),
          child: ElevatedButton(
            onPressed: onPlaceOrder,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.transparent,
              shadowColor: Colors.transparent,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12)),
              padding: const EdgeInsets.symmetric(vertical: 14),
            ),
            child: onPlaceOrder == null
                ? const SizedBox(
                    width: 22,
                    height: 22,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Colors.white),
                  )
                : Text(
                    'Place Order  •  \$${total.toStringAsFixed(2)}',
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 15,
                        fontWeight: FontWeight.w700),
                  ),
          ),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Placing order loading dialog
// ─────────────────────────────────────────────────────────────────────────────

class _PlacingOrderDialog extends StatelessWidget {
  const _PlacingOrderDialog();

  @override
  Widget build(BuildContext context) {
    return const Dialog(
      backgroundColor: Color(0xFF111111),
      shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.all(Radius.circular(16))),
      child: Padding(
        padding: EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            CircularProgressIndicator(
                color: Color(0xFFFF2D55), strokeWidth: 2.5),
            SizedBox(height: 20),
            Text('Placing your order...',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 15,
                    fontWeight: FontWeight.w500)),
            SizedBox(height: 4),
            Text('Please wait a moment.',
                style: TextStyle(color: Color(0xFF888888), fontSize: 13)),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Order success screen
// ─────────────────────────────────────────────────────────────────────────────

class _OrderSuccessScreen extends StatefulWidget {
  const _OrderSuccessScreen({required this.order});
  final OrderEntity order;

  @override
  State<_OrderSuccessScreen> createState() => _OrderSuccessScreenState();
}

class _OrderSuccessScreenState extends State<_OrderSuccessScreen>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _scaleAnim;
  late final Animation<double> _fadeAnim;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    _scaleAnim = CurvedAnimation(parent: _ctrl, curve: Curves.elasticOut);
    _fadeAnim = CurvedAnimation(parent: _ctrl, curve: Curves.easeIn);
    _ctrl.forward();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Estimated delivery: 5 days from now
    final eta = DateTime.now().add(const Duration(days: 5));
    final etaStr =
        '${eta.day} ${_monthName(eta.month)} ${eta.year}';

    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: FadeTransition(
              opacity: _fadeAnim,
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  // Animated checkmark
                  ScaleTransition(
                    scale: _scaleAnim,
                    child: Container(
                      width: 96,
                      height: 96,
                      decoration: const BoxDecoration(
                        color: Color(0xFF0D2E1A),
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(Icons.check,
                          color: Color(0xFF2ECC71), size: 52),
                    ),
                  ),
                  const SizedBox(height: 28),
                  const Text(
                    'Order Placed!',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 26,
                        fontWeight: FontWeight.w800),
                  ),
                  const SizedBox(height: 10),
                  Text(
                    'Order ID: #${widget.order.id.substring(0, 8).toUpperCase()}',
                    style: const TextStyle(
                        color: Color(0xFF888888), fontSize: 14),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    'Estimated delivery by $etaStr',
                    style: const TextStyle(
                        color: Color(0xFFAAAAAA), fontSize: 14),
                  ),
                  const SizedBox(height: 40),
                  SizedBox(
                    width: double.infinity,
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        gradient: const LinearGradient(
                          colors: [Color(0xFFFF2D55), Color(0xFFFF6B35)],
                        ),
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: ElevatedButton(
                        onPressed: () => Navigator.of(context)
                            .pushNamedAndRemoveUntil(
                                '/orders', (r) => r.isFirst),
                        style: ElevatedButton.styleFrom(
                          backgroundColor: Colors.transparent,
                          shadowColor: Colors.transparent,
                          shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(12)),
                          padding:
                              const EdgeInsets.symmetric(vertical: 14),
                        ),
                        child: const Text(
                          'Track My Order',
                          style: TextStyle(
                              color: Colors.white,
                              fontSize: 15,
                              fontWeight: FontWeight.w700),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextButton(
                    onPressed: () => Navigator.of(context)
                        .pushNamedAndRemoveUntil('/', (r) => false),
                    child: const Text('Continue Shopping',
                        style: TextStyle(
                            color: Color(0xFF888888), fontSize: 14)),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  String _monthName(int month) {
    const names = [
      'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
    ];
    return names[month - 1];
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Address form bottom sheet
// ─────────────────────────────────────────────────────────────────────────────

class _AddressForm extends StatefulWidget {
  const _AddressForm();

  @override
  State<_AddressForm> createState() => _AddressFormState();
}

class _AddressFormState extends State<_AddressForm> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _line1Ctrl = TextEditingController();
  final _line2Ctrl = TextEditingController();
  final _cityCtrl = TextEditingController();
  final _stateCtrl = TextEditingController();
  final _postalCtrl = TextEditingController();
  final _countryCtrl = TextEditingController(text: 'US');

  @override
  void dispose() {
    for (final c in [
      _nameCtrl, _phoneCtrl, _line1Ctrl, _line2Ctrl,
      _cityCtrl, _stateCtrl, _postalCtrl, _countryCtrl,
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  void _submit() {
    if (!_formKey.currentState!.validate()) return;
    final addr = BuyerInfoEntity(
      name: _nameCtrl.text.trim(),
      phone: _phoneCtrl.text.trim(),
      addressLine1: _line1Ctrl.text.trim(),
      addressLine2: _line2Ctrl.text.trim().isEmpty
          ? null
          : _line2Ctrl.text.trim(),
      city: _cityCtrl.text.trim(),
      state: _stateCtrl.text.trim(),
      postalCode: _postalCtrl.text.trim(),
      country: _countryCtrl.text.trim(),
    );
    Navigator.of(context).pop(addr);
  }

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).viewInsets.bottom;
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 20, 20, 20 + bottomPadding),
      child: Form(
        key: _formKey,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('Delivery Address',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w700)),
              const SizedBox(height: 16),
              _Field(controller: _nameCtrl, label: 'Full Name', required: true),
              _Field(controller: _phoneCtrl, label: 'Phone', required: true,
                  keyboardType: TextInputType.phone),
              _Field(controller: _line1Ctrl, label: 'Address Line 1', required: true),
              _Field(controller: _line2Ctrl, label: 'Address Line 2 (optional)'),
              Row(children: [
                Expanded(child: _Field(controller: _cityCtrl, label: 'City', required: true)),
                const SizedBox(width: 10),
                Expanded(child: _Field(controller: _stateCtrl, label: 'State', required: true)),
              ]),
              Row(children: [
                Expanded(child: _Field(controller: _postalCtrl, label: 'ZIP', required: true,
                    keyboardType: TextInputType.number)),
                const SizedBox(width: 10),
                Expanded(child: _Field(controller: _countryCtrl, label: 'Country', required: true)),
              ]),
              const SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _submit,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFFFF2D55),
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10)),
                    padding: const EdgeInsets.symmetric(vertical: 14),
                  ),
                  child: const Text('Save Address',
                      style: TextStyle(fontWeight: FontWeight.w600)),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Field extends StatelessWidget {
  const _Field({
    required this.controller,
    required this.label,
    this.required = false,
    this.keyboardType,
  });

  final TextEditingController controller;
  final String label;
  final bool required;
  final TextInputType? keyboardType;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: TextFormField(
        controller: controller,
        keyboardType: keyboardType,
        style: const TextStyle(color: Colors.white, fontSize: 14),
        decoration: InputDecoration(
          labelText: label,
          labelStyle: const TextStyle(color: Color(0xFF888888), fontSize: 13),
          filled: true,
          fillColor: const Color(0xFF1A1A1A),
          contentPadding:
              const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: const BorderSide(color: Color(0xFF2A2A2A)),
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: const BorderSide(color: Color(0xFF2A2A2A)),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: const BorderSide(color: Color(0xFFFF2D55)),
          ),
          errorBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide:
                const BorderSide(color: Color(0xFFFF2D55), width: 0.5),
          ),
          focusedErrorBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: const BorderSide(color: Color(0xFFFF2D55)),
          ),
        ),
        validator: required
            ? (v) => (v == null || v.trim().isEmpty) ? 'Required' : null
            : null,
      ),
    );
  }
}
