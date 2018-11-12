{
  uid                   = 1001;
  hashedPassword = "$6$Gd8OTKhra5aNlVTS$vr7eA8XAT5/zihcIxST8QztwS9.LcrDKvFT4gzQTVYSB/9tSD/BFRK5otiZguqYpG9II.xdrpL7Ny3Lr86VnU/";
  # add ssh keys here
  openssh.authorizedKeys.keys = [

  ];
  isNormalUser = true;
  extraGroups  = [ "wheel" ];
}
