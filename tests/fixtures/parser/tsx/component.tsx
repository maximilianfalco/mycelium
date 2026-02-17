// @ts-nocheck
import React from "react";

interface ButtonProps {
  label: string;
  onClick: () => void;
}

const Button: React.FC<ButtonProps> = ({ label, onClick }) => {
  return <button onClick={onClick}>{label}</button>;
};

class Counter extends React.Component<{}, { count: number }> {
  constructor(props: {}) {
    super(props);
    this.state = { count: 0 };
  }

  increment(): void {
    this.setState({ count: this.state.count + 1 });
  }

  render() {
    return <div>{this.state.count}</div>;
  }
}
